package db

import (
	"context"
	"fmt"
	"time"

	"github.com/fish-tennis/gentity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// 账号区服注册表:保证一个账号在同一个区服只能创建1个角色
//
// 为什么用独立注册表,而不是直接在player集合上建AccountId+RegionId复合唯一索引:
// player集合是分片集合(分片键为_id hashed),MongoDB要求分片集合上的唯一索引
// 必须以分片键为前缀,AccountId+RegionId不含分片键_id,该索引在分片集群上无法创建;
// 注册表集合的_id直接编码为"{accountId}_{regionId}",分片键即业务键,
// 相同_id必然路由到同一个shard,由该shard原子拒绝重复插入,
// 唯一性由MongoDB全局保证,天然规避分片限制。
// 单机/副本集部署时该方案同样正确,一套代码两种环境通用。
//
// _id用下划线分隔的字符串而非数字拼接:accountId与regionId值域可能重叠,
// 数字拼接(如accountId*10000+regionId)会产生歧义碰撞
//
// 注册表定位为"带过期保护的建角互斥锁",账号->角色的持久事实以player表为准:
//  1. 预检查player表(存量兜底+TTL清理后的最终裁决,见下方crash恢复说明)
//  2. 原子抢占注册表:insert成功者获得建角资格,E11000说明有并发(见gameserver的处理)
//  3. player表插入成功后删除注册位(尽力而为,失败靠TTL兜底)
//
// crash恢复(消除注册位永久泄漏):
// 抢占成功后、player写入完成前进程crash,回滚代码不会执行,注册位成为残留;
// 注册文档带ExpireAt字段+TTL索引,MongoDB后台线程(默认每60秒扫描一次)会自动删除
// 过期残留(总延迟约1~2分钟),之后该账号可重新建角。
// TTL误删不会破坏唯一性约束:残留被清理后,重试者仍需通过player表预检查裁决,
// player已插入则拒绝,未插入才放行,两道检查互补,不存在双角色窗口。
// ExpireAt保护期须远大于建角耗时(毫秒级),否则TTL可能删掉进行中的注册,
// 极端情况下窗口重现,故取120秒宽裕值。

// PlayerAccountLockTtl 注册位保护期:覆盖建角全程,超时后允许TTL索引清理残留
const PlayerAccountLockTtl = 120 * time.Second

// ExpireAtFieldName TTL索引字段名:值为绝对过期时刻(BSON日期),expireAfterSeconds=0
const ExpireAtFieldName = "ExpireAt"

// GenPlayerAccountKey 生成注册表的唯一key
// 格式"{accountId}_{regionId}":下划线分隔保证不同的账号/区服组合不会碰撞
func GenPlayerAccountKey(accountId int64, regionId int32) string {
	return fmt.Sprintf("%d_%d", accountId, regionId)
}

// GetPlayerAccountDb 注册表集合访问入口
func GetPlayerAccountDb() gentity.EntityDb {
	return _dbMgr.GetEntityDb(PlayerAccountDbName).(gentity.EntityDb)
}

// GetPlayerAccountCollection 获取注册表的原生MongoDB Collection
// 创建TTL索引等管理操作需要通过原生driver执行
func GetPlayerAccountCollection() *mongo.Collection {
	return GetDbMgr().GetEntityDb(PlayerAccountDbName).(*gentity.MongoCollection).GetCollection()
}

// EnsurePlayerAccountTtlIndex 创建注册表的TTL索引
// expireAfterSeconds=0表示文档在ExpireAt字段指定的时刻过期删除,
// 用于自动清理建角中断(crash/panic)留下的注册位残留;
// createIndex对相同spec幂等,启动时反复调用安全
func EnsurePlayerAccountTtlIndex() error {
	model := mongo.IndexModel{
		Keys:    bson.D{{Key: ExpireAtFieldName, Value: 1}},
		Options: options.Index().SetExpireAfterSeconds(int32(0)),
	}
	_, err := GetPlayerAccountCollection().Indexes().CreateOne(context.Background(), model)
	return err
}

// RegisterPlayerAccount 建角前原子抢占账号区服注册位
// 返回(false,nil)表示抢占失败(并发建角或残留未过期),由调用方查player表区分具体原因;
// 返回(false,err)表示数据库异常,应视为建角失败
func RegisterPlayerAccount(accountId int64, regionId int32, playerId int64) (bool, error) {
	key := GenPlayerAccountKey(accountId, regionId)
	// InsertEntity内部即InsertOne,_id重复时MongoDB原子返回E11000,
	// 这是并发建角场景下的最终防线:两个请求同时通过预检查,只允许一个插入成功
	err, isDuplicateKey := GetPlayerAccountDb().InsertEntity(key, map[string]any{
		UniqueIdName:    key,
		PlayerIdKeyName: playerId,
		// 记录抢占时间,用于排查建角中断(抢占成功但player未写入)类问题
		"CreateTime": time.Now().Unix(),
		// 过期时刻:TTL索引据此清理crash残留,建角成功后文档会被主动删除,
		// 正常流程不会等到过期
		ExpireAtFieldName: time.Now().Add(PlayerAccountLockTtl),
	})
	if isDuplicateKey {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UnregisterPlayerAccount 释放注册位
// 两个调用时机:
//  1. 注册位抢占成功后、player集合写入失败时(回滚,否则要等TTL过期才能重试)
//  2. 建角成功后(注册表仅作互斥锁,事实以player表为准)
// 删除失败无需重试:残留文档会被TTL索引自动清理
func UnregisterPlayerAccount(accountId int64, regionId int32) error {
	return GetPlayerAccountDb().DeleteEntity(GenPlayerAccountKey(accountId, regionId))
}

package db

import (
	"context"
	"fmt"

	"github.com/fish-tennis/gentity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// 账号区服→玩家 持久映射表:一张表同时解决两个问题
//
// 1. 建角防重:一个账号在同一个区服只能创建1个角色
// 为什么用独立表,而不是直接在player集合上建AccountId+RegionId复合唯一索引:
// player集合是分片集合(分片键为_id hashed),MongoDB要求分片集合上的唯一索引
// 必须以分片键为前缀,AccountId+RegionId不含分片键_id,该索引在分片集群上无法创建;
// 映射表_id直接编码为"{accountId}_{regionId}",相同_id必然路由到同一个shard,
// 由该shard原子拒绝重复插入,唯一性由MongoDB全局保证,天然规避分片限制。
// 单机/副本集部署时该方案同样正确,一套代码两种环境通用。
//
// 2. 按账号查角色的直达入口
// player表分片键为_id(playerId),按AccountId查询会被mongos广播到所有分片;
// 映射表按_id(即账号区服键)查询直达,登录/进游链路先查映射拿到playerId,
// 再按_id操作player表,全程无广播。
//
// 映射是持久事实:
//   - 建角:insert映射,_id冲突=该账号该区服已有角色,原子裁决,无双角色窗口,
//   - player表写入失败:回滚delete映射,可立即重试
//   - 建角成功后映射永久保留
//   - 映射不可变(playerId与accountId的绑定建角后终身不变),无一致性难题,
//     可放心叠加redis/进程内缓存以消除常态查询开销

// GenAccountPlayerKey 生成映射表的唯一key
// 格式"{accountId}_{regionId}":下划线分隔保证不同的账号/区服组合不会碰撞
func GenAccountPlayerKey(accountId int64, regionId int32) string {
	return fmt.Sprintf("%d_%d", accountId, regionId)
}

// AccountPlayerMapData 映射文档(不含_id字段,通过key读写)
type AccountPlayerMapData struct {
	PlayerId  int64 `bson:"PlayerId"`
	AccountId int64 `bson:"AccountId"`
	RegionId  int32 `bson:"RegionId"`
}

// GetAccountPlayerDb 映射表集合访问入口
func GetAccountPlayerDb() gentity.EntityDb {
	return _dbMgr.GetEntityDb(AccountPlayerDbName).(gentity.EntityDb)
}

// GetAccountPlayerCollection 获取映射表的原生MongoDB Collection
// 范围查询等原生操作需要通过原生driver执行
func GetAccountPlayerCollection() *mongo.Collection {
	return GetDbMgr().GetEntityDb(AccountPlayerDbName).(*gentity.MongoCollection).GetCollection()
}

// FindPlayerIdByAccount 查账号在该区服的角色id(player分片集群下按账号查玩家的直达入口)
// 返回(0,nil)表示该账号在此区服没有角色
// NOTE:精确查_id直达,替代player表的FindPlayerIdByAccountId(分片集群下广播所有分片)
func FindPlayerIdByAccount(accountId int64, regionId int32) (int64, error) {
	data := &AccountPlayerMapData{}
	found, err := GetAccountPlayerDb().FindEntityById(GenAccountPlayerKey(accountId, regionId), data)
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	return data.PlayerId, nil
}

// FindAccountPlayersByAccount 查账号在所有区服的角色映射(登录角色列表入口)
// 用_id前缀范围查询而非AccountId字段查询:主键前缀天然有索引,
// 且将来若映射表分片(ranged _id),前缀查询仍可路由直达,字段查询会广播
func FindAccountPlayersByAccount(accountId int64) ([]*AccountPlayerMapData, error) {
	// 范围[prefix, prefix+"\xff")精确匹配前缀:accountId/regionId值域(数字+下划线)
	// 均小于\xff,不会误包含更长key,也不会漏掉(如"1234_.."因'4'<'_'天然落在范围外)
	prefix := fmt.Sprintf("%d_", accountId)
	cursor, err := GetAccountPlayerCollection().Find(context.Background(), bson.D{
		{Key: UniqueIdName, Value: bson.D{
			{Key: "$gte", Value: prefix},
			{Key: "$lt", Value: prefix + "\xff"},
		}},
	})
	if err != nil {
		return nil, err
	}
	var results []*AccountPlayerMapData
	if err = cursor.All(context.Background(), &results); err != nil {
		return nil, err
	}
	return results, nil
}

// InsertAccountPlayerMap 建角时写入映射(原子防重)
// 返回(false,nil)表示该账号在该区服已有角色(_id冲突,由MongoDB原子裁决);
// 返回(false,err)表示数据库异常,应视为建角失败
// 并发建角(双端登录/请求重试/恶意刷)时,只有insert成功的请求能继续,其余被数据库层拒绝
func InsertAccountPlayerMap(accountId int64, regionId int32, playerId int64) (bool, error) {
	key := GenAccountPlayerKey(accountId, regionId)
	err, isDuplicateKey := GetAccountPlayerDb().InsertEntity(key, map[string]any{
		UniqueIdName:     key,
		PlayerIdKeyName:  playerId,
		AccountIdKeyName: accountId,
		PlayerRegionId:   regionId,
	})
	if isDuplicateKey {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteAccountPlayerMap 删除映射
// 调用时机:player表写入失败时的建角回滚(可立即重试);未来删号功能
func DeleteAccountPlayerMap(accountId int64, regionId int32) error {
	return GetAccountPlayerDb().DeleteEntity(GenAccountPlayerKey(accountId, regionId))
}

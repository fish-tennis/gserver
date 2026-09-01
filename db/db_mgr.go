package db

import (
	"context"

	"github.com/fish-tennis/gentity"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

const (
	AccountDbName = "account" // 账号数据库名
	PlayerDbName  = "player"  // 玩家数据库名
	GuildDbName   = "guild"   // 公会数据库名
	GlobalDbName  = "global"  // 全局数据库名
	BanDbName           = "ban"            // 封禁记录数据库名
	// 账号区服注册表集合名:一个账号在同一个区服只能创建1个角色的原子防重集合
	// (见player_account.go的设计说明)
	PlayerAccountDbName = "player_account"
	UniqueIdName  = "_id"     // 数据库id列名

	AccountIdKeyName  = "AccountId"
	PlayerIdKeyName   = "PlayerId"
	GuildIdKeyName    = "GuildId"
	GlobalDbKeyName   = "Key"
	GlobalDbValueName = "Value" // global表作为kv数据库时的value列名
	RegionsKeyName    = "Regions" // global表中区服列表的key

	// account表里的固定字段
	AccountName = "Name"

	// player表里的几个固定字段
	PlayerName      = "Name"
	PlayerAccountId = "AccountId"
	PlayerRegionId  = "RegionId"
)

var (
	// singleton
	// 玩家数据接口
	// https://github.com/uber-go/guide/blob/master/style.md#prefix-unexported-globals-with-_
	_dbMgr gentity.DbMgr
)

func SetDbMgr(dbMgr gentity.DbMgr) {
	_dbMgr = dbMgr
}

func GetDbMgr() gentity.DbMgr {
	return _dbMgr
}

// 玩家数据库
func GetPlayerDb() gentity.PlayerDb {
	return _dbMgr.GetEntityDb(PlayerDbName).(gentity.PlayerDb)
}

// 公会数据库
func GetGuildDb() gentity.EntityDb {
	return _dbMgr.GetEntityDb(GuildDbName).(gentity.EntityDb)
}

// global数据库
func GetGlobalDb() gentity.EntityDb {
	return _dbMgr.GetEntityDb(GlobalDbName).(gentity.EntityDb)
}

// global数据库同时也是kv数据库
func GetKvDb() gentity.KvDb {
	return _dbMgr.GetKvDb(GlobalDbName).(gentity.KvDb)
}

// EnsurePlayerAccountRegionIndex 创建player集合的{AccountId,RegionId}复合索引(非唯一)
//
// 服务的查询(均不带分片键_id,分片集群下会扇出所有shard,更需要每shard上有索引直达):
//   - 登录时queryAccountRegionRoles按AccountId查角色列表(每次登录必经)
//   - 进服/建角预检查的FindPlayerIdByAccountId按{AccountId,RegionId}查
//   - GM后台按账号ID搜玩家
// 无此索引时上述查询在每shard上退化为全集合扫描,玩家量增长后登录链路会被拖垮;
// 唯一性由player_account注册表承担,此索引仅作查询加速(非唯一,分片集群创建合法);
// 框架的CreateIndex只支持单列,复合索引走原生IndexModel
func EnsurePlayerAccountRegionIndex() error {
	model := mongo.IndexModel{
		Keys: bson.D{
			{Key: PlayerAccountId, Value: 1},
			{Key: PlayerRegionId, Value: 1},
		},
	}
	_, err := GetPlayerDb().(interface {
		GetCollection() *mongo.Collection
	}).GetCollection().Indexes().CreateOne(context.Background(), model)
	return err
}

package db

import "github.com/fish-tennis/gentity"

const (
	AccountDbName = "account" // 账号数据库名
	PlayerDbName  = "player"  // 玩家数据库名
	GuildDbName   = "guild"   // 公会数据库名
	GlobalDbName  = "global"  // 全局数据库名
	UniqueIdName  = "_id"     // 数据库id列名

	AccountIdKeyName  = "AccountId"
	PlayerIdKeyName   = "PlayerId"
	GuildIdKeyName    = "GuildId"
	GlobalDbKeyName   = "Key"
	GlobalDbValueName = "Value" // global表作为kv数据库时的value列名

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

// 各个表的注册统一放在公共的db目录下,分片策略在这里统一定义
// 避免不同的服务器注册同一个表时,使用了不同的ShardKeyType

// 注册账号数据库
func RegisterAccountDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(AccountDbName, gentity.ShardKeyNone, UniqueIdName)
}

// 注册玩家数据库
func RegisterPlayerDb(mongoDb *gentity.MongoDb) gentity.PlayerDb {
	return mongoDb.RegisterPlayerDb(PlayerDbName, gentity.ShardKeyHashed, UniqueIdName, PlayerAccountId, PlayerRegionId)
}

// 注册公会数据库
func RegisterGuildDb(mongoDb *gentity.MongoDb) gentity.EntityDb {
	return mongoDb.RegisterEntityDb(GuildDbName, gentity.ShardKeyNone, UniqueIdName)
}

// 注册全局对象数据库(如GlobalEntity),同时也是kv数据库
func RegisterGlobalDb(mongoDb *gentity.MongoDb) {
	mongoDb.RegisterEntityDb(GlobalDbName, gentity.ShardKeyNone, GlobalDbKeyName)
	mongoDb.RegisterKvDb(GlobalDbName, gentity.ShardKeyNone, GlobalDbKeyName, GlobalDbValueName)
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

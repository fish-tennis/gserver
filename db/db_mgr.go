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
	GlobalDbKeyName   = "key"
	GlobalDbValueName = "value" // global表作为kv数据库时的value列名
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

package db

import "github.com/fish-tennis/gentity"

const (
	KvDbName         = "kv" // kv数据库名
	KvKeyName        = "k"
	KvValueName      = "v"
	AccountIdKeyName = "AccountId"
	PlayerIdKeyName  = "PlayerId"
	GuildIdKeyName   = "GuildId"

	AccountDbName = "account" // 账号数据库名
	PlayerDbName  = "player"  // 玩家数据库名
	GuildDbName   = "guild"   // 公会数据库名
	UniqueIdName  = "_id"     // 数据库id列名
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

// 玩家数据表
func GetPlayerDb() gentity.PlayerDb {
	return _dbMgr.GetEntityDb(PlayerDbName).(gentity.PlayerDb)
}

package db

import "github.com/fish-tennis/gentity"

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
	return _dbMgr.GetEntityDb("player").(gentity.PlayerDb)
}
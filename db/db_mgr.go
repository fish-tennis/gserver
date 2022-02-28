package db

var (
	// singleton
	// 玩家数据接口
	// https://github.com/uber-go/guide/blob/master/style.md#prefix-unexported-globals-with-_
	_dbMgr DbMgr
)

func SetDbMgr(dbMgr DbMgr) {
	_dbMgr = dbMgr
}

func GetDbMgr() DbMgr {
	return _dbMgr
}

// 玩家数据表
func GetPlayerDb() PlayerDb {
	return _dbMgr.GetEntityDb("player").(PlayerDb)
}
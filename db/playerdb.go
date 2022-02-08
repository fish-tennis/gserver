package db

var (
	// singleton
	// 玩家数据接口
	_playerDb PlayerDb
)

func SetPlayerDb(playerDb PlayerDb) {
	_playerDb = playerDb
}

func GetPlayerDb() PlayerDb {
	return _playerDb
}
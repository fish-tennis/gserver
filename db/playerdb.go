package db

var (
	// singleton
	// 玩家数据接口
	// https://github.com/uber-go/guide/blob/master/style.md#prefix-unexported-globals-with-_
	_playerDb PlayerDb
)

func SetPlayerDb(playerDb PlayerDb) {
	_playerDb = playerDb
}

func GetPlayerDb() PlayerDb {
	return _playerDb
}
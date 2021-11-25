package db

// 数据接口
// 游戏应用里,核心数据有账号和玩家数据,都是KV存储
type Db interface {
	// 加载数据(find by string key)
	FindString(key string, data interface{}) (bool,error)

	// 新建数据(insert by string key)
	InsertString(key string, data interface{}) error

	// 保存数据(update by string key)
	UpdateString(key string, data interface{}) error

	// 加载数据(find by int key)
	FindInt64(key int64, data interface{}) (bool,error)

	// 新建数据(insert by int key)
	InsertInt64(key int64, data interface{}) error

	// 保存数据(update by int key)
	UpdateInt64(key int64, data interface{}) error
}

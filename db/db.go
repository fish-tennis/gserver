package db

// 数据接口
// 游戏应用里,核心数据有账号和玩家数据,都是Key-Value存储
// Db接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
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

	// 加载数据(find by int key.fieldName)
	LoadFieldInt64(key int64, fieldName string, fieldData interface{}) (bool,error)

	// 保存字段(upsert by int key.fieldName)
	SaveFieldInt64(key int64, fieldName string, fieldData interface{}) error
}

// 玩家数据接口
type PlayerDb interface {
	// 根据账号id查找玩家数据
	// 适用于一个账号在一个区服只有一个玩家角色的游戏
	FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool,error)

	// 新建玩家
	InsertPlayer(playerId int64, playerData interface{}) error

	// 保存玩家组件(update by int playerId.componentName)
	SaveComponent(playerId int64, componentName string, componentData interface{}) error
}

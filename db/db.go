package db

// 账号数据接口
// Db接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
type AccountDb interface {
	// 根据账号名查找账号数据
	FindAccount(accountName string, data interface{}) (bool,error)

	// 新建账号(insert)
	InsertAccount(accountData interface{}) (err error, isDuplicateKey bool)

	// 保存账号数据(update account by accountId)
	SaveAccount(accountId int64, accountData interface{}) error

	// 保存账号字段(update account.fieldName by accountId)
	SaveAccountField(accountId int64, fieldName string, fieldData interface{}) error
}

// 玩家数据接口
// Db接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
type PlayerDb interface {
	// 根据账号id查找玩家数据
	// 适用于一个账号在一个区服只有一个玩家角色的游戏
	FindPlayerByAccountId(accountId int64, regionId int32, playerData interface{}) (bool,error)

	// 新建玩家(insert)
	InsertPlayer(playerId int64, playerData interface{}) (err error, isDuplicateKey bool)

	// 保存玩家数据(update player by playerId)
	SavePlayer(playerId int64, playerData interface{}) error

	// 保存玩家1个组件(update player's component)
	SaveComponent(playerId int64, componentName string, componentData interface{}) error

	// 批量保存玩家组件(update player's components...)
	SaveComponents(playerId int64, components map[string]interface{}) error

	// 保存玩家1个组件的一个字段(update player's component.field)
	SaveComponentField(playerId int64, componentName string, fieldName string, fieldData interface{}) error
}

// Kv数据接口
// 游戏应用里,除了账号数据和玩家数据之外,其他以Key-Value存储的数据
// Db接口是为了应用层能够灵活的更换存储数据库(mysql,mongo,redis等)
type KvDb interface {
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

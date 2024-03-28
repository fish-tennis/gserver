package game

var (
	_globalEntity *GlobalEntity
)

func GetGlobalEntity() *GlobalEntity {
	return _globalEntity
}

type Hook struct {
}

// 服务器初始化回调
func (h *Hook) OnApplicationInit(initArg interface{}) {
	InitGlobalEntityStructAndHandler()
	_globalEntity = CreateGlobalEntityFromDb()
	_globalEntity.GetProcessStatInfo().OnStartup()
	_globalEntity.checkDataDirty()
}

// 服务器关闭回调
func (h *Hook) OnApplicationExit() {
	_globalEntity.GetProcessStatInfo().OnShutdown()
	_globalEntity.SaveDb(true)
}

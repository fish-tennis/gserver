package player

var(
	// 玩家组件名和组件索引的对照表
	// 玩家的结构是固定的,所以这个对照表可以共用
	_playerComponentNameMap map[string]int
)

func InitPlayerComponentMap() {
	_playerComponentNameMap = make(map[string]int)
	player := CreateTempPlayer(0,0)
	for idx,component := range player.components {
		_playerComponentNameMap[component.GetName()] = idx
	}
}

func GetComponentIndex(componentName string) int {
	if index,ok := _playerComponentNameMap[componentName]; ok {
		return index
	}
	return -1
}

func GetPlayerComponentMap() map[string]int {
	return _playerComponentNameMap
}
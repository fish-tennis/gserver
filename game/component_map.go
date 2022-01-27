package game

var(
	// 玩家组件名和组件索引的对照表
	// 玩家的结构是固定的,所以这个对照表可以共用
	playerComponentNameMap map[string]int
)

func InitPlayerComponentMap() {
	playerComponentNameMap = make(map[string]int)
	player := createTempPlayer()
	for idx,component := range player.components {
		playerComponentNameMap[component.GetName()] = idx
	}
}

func GetComponentIndex(componentName string) int {
	if index,ok := playerComponentNameMap[componentName]; ok {
		return index
	}
	return -1
}
package game

import "github.com/fish-tennis/gserver/logger"

// 提供一个统一的属性值查询接口
func (this *Player) GetPropertyInt32(propertyName string) int32 {
	switch propertyName {
	case "Level":
		return this.GetBaseInfo().Data.GetLevel()
	case "TotalPay":
		return this.GetBaseInfo().Data.GetTotalPay()
	default:
		logger.Error("Not support property %v %v", this.GetId(), propertyName)
	}
	return 0
}

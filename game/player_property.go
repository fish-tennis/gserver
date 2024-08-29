package game

import "github.com/fish-tennis/gserver/logger"

var _playerPropertyGetterMap map[string]PlayerPropertyGetter

func init() {
	_playerPropertyGetterMap = map[string]PlayerPropertyGetter{
		"Level": func(p *Player, propertyName string) int32 {
			return p.GetBaseInfo().Data.GetLevel()
		},
		"TotalPay": func(p *Player, propertyName string) int32 {
			return p.GetBaseInfo().Data.GetTotalPay()
		},
		"OnlineSecond": func(p *Player, propertyName string) int32 {
			return p.GetBaseInfo().Data.GetTotalOnlineSeconds()
		},
		"OnlineMinute": func(p *Player, propertyName string) int32 {
			return p.GetBaseInfo().GetTotalOnlineSeconds() / 60
		},
		// 签到特殊处理,用于条件初始化时
		"SignIn": func(p *Player, propertyName string) int32 {
			return 0
		},
	}
}

type PlayerPropertyGetter func(p *Player, propertyName string) int32

// 提供一个统一的属性值查询接口
func (p *Player) GetPropertyInt32(propertyName string) int32 {
	if getter, ok := _playerPropertyGetterMap[propertyName]; ok {
		return getter(p, propertyName)
	}
	logger.Error("Not support property %v %v", p.GetId(), propertyName)
	return 0
}

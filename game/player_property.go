package game

import (
	"log/slog"

	"github.com/fish-tennis/gserver/pb"
)

var _playerPropertyGetterMap map[string]PlayerPropertyGetter

func init() {
	_playerPropertyGetterMap = map[string]PlayerPropertyGetter{
		"Level": func(p *Player, propertyName string, _ *pb.ConditionCfg) int32 {
			return p.GetBaseInfo().Data.GetLevel()
		},
		"TotalPay": func(p *Player, propertyName string, _ *pb.ConditionCfg) int32 {
			return p.GetBaseInfo().Data.GetTotalPay()
		},
		"OnlineSecond": func(p *Player, propertyName string, _ *pb.ConditionCfg) int32 {
			return p.GetBaseInfo().Data.GetTotalOnlineSeconds()
		},
		"OnlineMinute": func(p *Player, propertyName string, _ *pb.ConditionCfg) int32 {
			return p.GetBaseInfo().GetTotalOnlineSeconds() / 60
		},
		// 建号天数: 用于按开服/创角天数触发的活动条件判断
		"CreateDayCount": func(p *Player, _ string, _ *pb.ConditionCfg) int32 {
			return p.GetBaseInfo().GetCreateDayCount()
		},
	}
}

type PlayerPropertyGetter func(p *Player, propertyName string, conditionCfg *pb.ConditionCfg) int32

// 提供一个统一的属性值查询接口
func (p *Player) GetPropertyInt32(propertyName string, conditionCfg *pb.ConditionCfg) int32 {
	if getter, ok := _playerPropertyGetterMap[propertyName]; ok {
		return getter(p, propertyName, conditionCfg)
	}
	slog.Error("Not support property", "playerId", p.GetId(), "propertyName", propertyName)
	return 0
}

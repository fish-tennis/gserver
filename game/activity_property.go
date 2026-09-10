package game

import (
	"log/slog"
	"time"

	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
)

type ActivityPropertyGetter func(a *ActivityDefault, propertyName string, conditionCfg *pb.ConditionCfg) int32

var _activityPropertyGetterMap map[string]ActivityPropertyGetter

func init() {
	_activityPropertyGetterMap = map[string]ActivityPropertyGetter{
		// 当前是参加这个活动的第几天,从1开始
		"DayCount": func(a *ActivityDefault, _ string, _ *pb.ConditionCfg) int32 {
			days := util.DayCount(a.Activities.GetPlayer().GetTimerEntries().Now(), time.Unix(int64(a.Base.JoinTime), 0))
			return int32(days) + 1
		},
	}
}

// 提供一个统一的属性值查询接口(专用于condition,属性名为string)
// 属性名优先在_activityPropertyGetterMap里注册,未注册的属性名则查pb.ActivityPropertyId枚举,
// 匹配的枚举值转化为Activity.GetProperty查询,实现服务器和客户端共用同一套属性名定义
// NOTE:条件接口是int32的,而Properties的属性值是int64的,int64的属性主要考虑活动自身逻辑的扩展需求,而不是条件和进度
func (a *ActivityDefault) GetPropertyInt32(propertyName string, conditionCfg *pb.ConditionCfg) int32 {
	if getter, ok := _activityPropertyGetterMap[propertyName]; ok {
		return getter(a, propertyName, conditionCfg)
	}
	if propertyId, ok := pb.ActivityPropertyId_value[propertyName]; ok {
		return int32(a.GetProperty(propertyId))
	}
	slog.Error("Not support property", "activityId", a.GetId(), "propertyName", propertyName)
	return 0
}

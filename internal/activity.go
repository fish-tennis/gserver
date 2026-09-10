package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"time"
)

type Activity interface {
	GetId() int32

	// 新活动初始化
	OnInit(t time.Time)

	// 响应事件
	OnEvent(event interface{})

	// 日期更新
	OnDateChange(oldDate time.Time, curDate time.Time)

	// 活动结束时的处理
	OnEnd(t time.Time)

	// 提供一个统一的属性值查询接口(专用于condition,属性名为string)
	GetPropertyInt32(propertyName string, conditionCfg *pb.ConditionCfg) int32

	// 获取活动数据上的动态属性值
	// propertyId为pb.ActivityPropertyId枚举值
	// NOTE:Properties的属性值是int64的,int64的属性主要考虑活动自身逻辑的扩展需求,而不是条件和进度
	GetProperty(propertyId int32) int64
}

type ActivityMgr interface {
	GetActivity(activityId int32) Activity
}

type BaseActivity struct {
	Id int32
}

func (this *BaseActivity) GetId() int32 {
	return this.Id
}

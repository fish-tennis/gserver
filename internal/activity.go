package internal

import "time"

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

	// 提供一个统一的属性值查询接口
	GetPropertyInt32(propertyName string) int32
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

package internal

import "time"

type Activity interface {
	GetId() int32
	OnEvent(event interface{})
	OnDateChange(oldDate time.Time, curDate time.Time)

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

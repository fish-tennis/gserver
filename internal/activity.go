package internal

type Activity interface {
	GetId() int32
	OnEvent(event interface{})
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

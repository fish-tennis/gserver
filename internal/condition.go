package internal

import (
	"reflect"
)

const (
	// 简单计数,每触发一次事件,进度+1
	// example:
	//   条件: 进行10场战斗
	//   每触发一次战斗事件,进度就+1
	ProgressType_Counter = 1

	// 每次事件触发时,重置进度
	// example:
	//   条件: 升到10级
	//   每触发一次升级事件,进度重置为当前等级
	ProgressType_Reset = 2
)

// 条件配置数据
type ConditionCfg struct {
	ConditionType int32      `json:"conditionType"` // 条件类型
	ProgressType  int32      `json:"progressType"`   // 进度类型
	Total         int32      `json:"total"`	// 总的进度要求
	EventArgs     map[string]interface{} `json:"eventArgs"` // 需要匹配的事件参数
}

// 进度读取接口
type ProgressHolder interface {
	GetProgress() int32
	SetProgress(progress int32)
}

type ConditionMgr struct {
	// 事件和条件的映射信息
	eventMapping map[reflect.Type]*eventMappingInfo

	// 条件关联的初始化接口
	// 例如:
	// 条件1: 玩家升到10级
	// 当初始化该条件时,条件进度用玩家当前等级来初始化
	conditionInits map[int32]ConditionInitFunc
}

func NewConditionMgr() *ConditionMgr {
	return &ConditionMgr{
		eventMapping: make(map[reflect.Type]*eventMappingInfo),
		conditionInits: make(map[int32]ConditionInitFunc),
	}
}

// 条件检查接口
// 返回事件触发该条件的进度
// example:
//   条件: 抽卡100次
//   事件: 抽卡(5连抽)
//   进度+5
type ConditionCheckFunc func(event interface{}, conditionCfg *ConditionCfg) int32

// 条件初始化接口
// 返回初始进度
type ConditionInitFunc func(arg interface{}, conditionCfg *ConditionCfg) int32

// 事件和条件的映射信息
type eventMappingInfo struct {
	// 事件类型
	eventTyp reflect.Type

	// 该事件关联的条件以及检查接口
	// 一个事件可以对应多个条件
	// 例如:
	// 条件1: 进行N场战斗
	// 条件2: 进行N场PVP战斗
	// 当触发战斗事件时,条件1会进度+1,但是条件2还需要进行更多的检查才能确定是否进度+1
	conditionCheckers map[int32]ConditionCheckFunc
}

// 注册事件和条件检查接口
// checker可以为nil
func (this *ConditionMgr) Register(conditionType int32, event interface{}, checker ConditionCheckFunc) {
	this.RegisterWithInit(conditionType, event, checker, nil)
}

// 注册事件和条件检查接口
// checker可以为nil
func (this *ConditionMgr) RegisterWithInit(conditionType int32, event interface{}, checker ConditionCheckFunc, init ConditionInitFunc) {
	eventTyp := reflect.TypeOf(event).Elem()
	info,ok := this.eventMapping[eventTyp]
	if !ok {
		info = &eventMappingInfo{
			eventTyp: eventTyp,
			conditionCheckers: make(map[int32]ConditionCheckFunc),
		}
		this.eventMapping[eventTyp] = info
	}
	info.conditionCheckers[conditionType] = checker
	if init != nil {
		this.conditionInits[conditionType] = init
	}
}

// 注册默认的条件检查接口
func (this *ConditionMgr) RegisterDefault(conditionType int32, event interface{}) {
	this.Register(conditionType, event, DefaultConditionChecker)
}

// 检查事件是否关联某个条件
func (this *ConditionMgr) IsMatchEvent(event interface{}, conditionType int32) bool {
	eventTyp := reflect.TypeOf(event).Elem()
	info,ok := this.eventMapping[eventTyp]
	if !ok {
		return false
	}
	_,has := info.conditionCheckers[conditionType]
	return has
}

// 获取事件,条件对应的检查接口
func (this *ConditionMgr) GetConditionChecker(event interface{}, conditionType int32) (ConditionCheckFunc,bool) {
	eventTyp := reflect.TypeOf(event).Elem()
	info,ok := this.eventMapping[eventTyp]
	if !ok {
		return nil,false
	}
	v,has := info.conditionCheckers[conditionType]
	return v,has
}

// 初始化条件,更新初始进度
func (this *ConditionMgr) InitCondition(arg interface{}, conditionCfg *ConditionCfg, progressHolder ProgressHolder) bool {
	initFunc,ok := this.conditionInits[conditionCfg.ConditionType]
	if !ok {
		return false
	}
	if initFunc == nil {
		return false
	}
	progress := initFunc(arg, conditionCfg)
	if progressHolder.GetProgress() != progress {
		progressHolder.SetProgress(progress)
		return true
	}
	return false
}

// 检查事件是否触发进度的更新,并更新进度
func (this *ConditionMgr) CheckEvent(event interface{}, conditionCfg *ConditionCfg, progressHolder ProgressHolder) bool {
	checker,ok := this.GetConditionChecker(event, conditionCfg.ConditionType)
	if !ok {
		return false
	}
	if progressHolder.GetProgress() >= conditionCfg.Total {
		return false
	}
	progress := int32(0)
	if checker != nil {
		progress = checker(event, conditionCfg)
	}
	if conditionCfg.ProgressType == ProgressType_Counter {
		progress = 1
	} else if conditionCfg.ProgressType == ProgressType_Reset {
		if progressHolder.GetProgress() != progress {
			progressHolder.SetProgress(progress)
			return true
		}
		return false
	}
	if progress > 0 {
		progressHolder.SetProgress(progressHolder.GetProgress()+progress)
		return true
	}
	return false
}

// 默认的条件检查接口
func DefaultConditionChecker(event interface{}, conditionCfg *ConditionCfg) int32 {
	if len(conditionCfg.EventArgs) == 0 {
		return 1
	}
	eventVal := reflect.ValueOf(event).Elem()
	// 匹配事件参数
	for fieldName,fieldValue := range conditionCfg.EventArgs {
		eventFieldVal := eventVal.FieldByName(fieldName)
		if !eventFieldVal.IsValid() {
			continue
		}
		switch eventFieldVal.Kind(){
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
			eventFieldInt := eventFieldVal.Int()
			conditionFieldInt,_ := fieldValue.(int) // json的值类型
			if eventFieldInt != int64(conditionFieldInt) {
				return 0
			}
		case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
			eventFieldInt := eventFieldVal.Uint()
			conditionFieldInt,_ := fieldValue.(int) // json的值类型
			if eventFieldInt != uint64(conditionFieldInt) {
				return 0
			}
		case reflect.Bool:
			eventFieldBool := eventFieldVal.Bool()
			conditionFieldInt,_ := fieldValue.(bool) // json的值类型
			if eventFieldBool != conditionFieldInt {
				return 0
			}
		case reflect.String:
			eventFieldStr := eventFieldVal.String()
			conditionFieldStr := fieldValue.(string)
			if eventFieldStr != conditionFieldStr {
				return 0
			}
		default:
			return 0
		}
	}
	return 1
}

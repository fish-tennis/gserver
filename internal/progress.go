package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
	"math"
	"reflect"
)

// 进度读取接口
type ProgressHolder interface {
	GetProgress() int32
	SetProgress(progress int32)
}

// 进度相关接口管理
type ProgressMgr struct {
	// 事件和进度的映射信息
	eventMapping map[reflect.Type]*eventMappingInfo

	// 进度初始化接口
	// 例如:
	// 进度1: 玩家升到10级
	// 当初始化该进度时,进度用玩家当前等级来初始化
	progressInits map[int32]ProgressInitFunc
}

func NewProgressMgr() *ProgressMgr {
	return &ProgressMgr{
		eventMapping:  make(map[reflect.Type]*eventMappingInfo),
		progressInits: make(map[int32]ProgressInitFunc),
	}
}

// 进度检查接口
// 返回事件触发时的进度
// example:
//
//	进度: 抽卡100次
//	事件: 抽卡(5连抽)
//	进度+5
type ProgressCheckFunc func(event any, progressCfg *pb.ProgressCfg) int32

// 进度初始化接口
// 返回初始进度
type ProgressInitFunc func(arg any, progressCfg *pb.ProgressCfg) int32

// 事件和进度的映射信息
type eventMappingInfo struct {
	// 事件类型
	eventTyp reflect.Type

	// 该事件关联的进度以及检查接口
	// 一个事件可以对应多个进度
	// 例如:
	// 进度1: 进行N场战斗
	// 进度2: 进行N场PVP战斗
	// 当触发战斗事件时,进度1会进度+1,但是进度2还需要进行更多的检查才能确定是否进度+1
	progressCheckers map[int32]ProgressCheckFunc
}

// 注册事件和进度检查接口
// checker可以为nil
func (m *ProgressMgr) Register(progressType int32, event any, checker ProgressCheckFunc) {
	m.RegisterWithInit(progressType, event, checker, nil)
}

// 注册事件和进度检查接口
//
//	checker: 进度检查接口,可以为nil
//	init: 初始化时,更新当前进度,可以为nil
func (m *ProgressMgr) RegisterWithInit(progressType int32, event any, checker ProgressCheckFunc, init ProgressInitFunc) {
	eventTyp := reflect.TypeOf(event).Elem()
	info, ok := m.eventMapping[eventTyp]
	if !ok {
		info = &eventMappingInfo{
			eventTyp:         eventTyp,
			progressCheckers: make(map[int32]ProgressCheckFunc),
		}
		m.eventMapping[eventTyp] = info
	}
	info.progressCheckers[progressType] = checker
	if init != nil {
		m.progressInits[progressType] = init
	}
}

// 注册默认的进度检查接口
func (m *ProgressMgr) RegisterDefault(progressType int32, event ...any) {
	for _, evt := range event {
		m.Register(progressType, evt, DefaultProgressChecker)
	}
}

// 检查事件是否关联某个进度
func (m *ProgressMgr) IsMatchEvent(event any, progressType int32) bool {
	eventTyp := reflect.TypeOf(event).Elem()
	info, ok := m.eventMapping[eventTyp]
	if !ok {
		return false
	}
	_, has := info.progressCheckers[progressType]
	return has
}

// 获取事件,进度对应的检查接口
func (m *ProgressMgr) GetProgressChecker(event any, progressType int32) (ProgressCheckFunc, bool) {
	eventTyp := reflect.TypeOf(event).Elem()
	info, ok := m.eventMapping[eventTyp]
	if !ok {
		return nil, false
	}
	v, has := info.progressCheckers[progressType]
	return v, has
}

// 初始化进度,更新初始进度
//
//	examples:
//	任务举例:玩家升级到10级,当5级玩家接任务时,初始进度就是5/10
func (m *ProgressMgr) InitProgress(arg any, progressCfg *pb.ProgressCfg, progressHolder ProgressHolder) bool {
	initFunc, ok := m.progressInits[progressCfg.Type]
	if !ok {
		return false
	}
	if initFunc == nil {
		return false
	}
	progress := initFunc(arg, progressCfg)
	if progressHolder.GetProgress() != progress {
		progressHolder.SetProgress(progress)
		return true
	}
	return false
}

// 检查事件是否触发进度的更新,并更新进度
func (m *ProgressMgr) CheckProgress(event any, progressCfg *pb.ProgressCfg, progressHolder ProgressHolder) bool {
	checker, ok := m.GetProgressChecker(event, progressCfg.Type)
	if !ok {
		return false
	}
	if progressHolder.GetProgress() >= progressCfg.Total {
		return false
	}
	progress := int32(0)
	if progressCfg.CountType == int32(pb.CountType_CountType_Counter) {
		progress = 1
	}
	if checker != nil {
		progress = checker(event, progressCfg)
	}
	if progressCfg.CountType == int32(pb.CountType_CountType_Reset) {
		if progressHolder.GetProgress() != progress {
			progressHolder.SetProgress(progress)
			return true
		}
		return false
	}
	if progress > 0 {
		progressHolder.SetProgress(progressHolder.GetProgress() + progress)
		return true
	}
	return false
}

// 默认的进度检查接口
func DefaultProgressChecker(event any, progressCfg *pb.ProgressCfg) int32 {
	// 通用事件匹配,先检查事件名是否匹配
	if progressCfg.GetType() == int32(pb.ProgressType_ProgressType_Event) {
		eventTyp := reflect.TypeOf(event)
		if eventTyp.Kind() == reflect.Pointer {
			eventTyp = eventTyp.Elem()
		}
		if eventTyp.Name() != "Event"+progressCfg.GetEvent() {
			return 0
		}
	}
	eventVal := reflect.ValueOf(event).Elem()
	// 匹配事件参数
	for fieldName, fieldValue := range progressCfg.Properties {
		eventFieldVal := eventVal.FieldByName(fieldName)
		if !eventFieldVal.IsValid() {
			return 0
		}
		switch eventFieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			eventFieldInt := eventFieldVal.Int()
			progressFieldInt := util.ToInt(fieldValue) // 兼容json和csv
			if eventFieldInt != int64(progressFieldInt) {
				return 0
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			eventFieldInt := eventFieldVal.Uint()
			progressFieldInt := util.ToUint(fieldValue) // // 兼容json和csv
			if eventFieldInt != uint64(progressFieldInt) {
				return 0
			}
		case reflect.Float32, reflect.Float64:
			eventFieldFloat := eventFieldVal.Float()
			progressFieldFloat := util.ToFloat(fieldValue) // 兼容json和csv
			// 浮点数比较大小,设置一个精度
			if math.Abs(eventFieldFloat-progressFieldFloat) >= 0.000001 {
				return 0
			}
		case reflect.Bool:
			eventFieldBool := eventFieldVal.Bool()
			progressFieldBool := util.ToBool(fieldValue) // 兼容json和csv
			if eventFieldBool != progressFieldBool {
				return 0
			}
		case reflect.String:
			eventFieldStr := eventFieldVal.String()
			if eventFieldStr != fieldValue {
				return 0
			}
		default:
			return 0
		}
	}
	// 检查progressCfg.EventField
	if progressCfg.GetCountType() == int32(pb.CountType_CountType_EventField) {
		eventFieldVal := eventVal.FieldByName(progressCfg.GetEventField())
		if !eventFieldVal.IsValid() {
			// event没有这个属性
			slog.Error("unsupported field", "name", progressCfg.GetEventField(), "event", event)
			return 0
		}
		switch eventFieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return int32(eventFieldVal.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return int32(eventFieldVal.Uint())
		case reflect.Float32, reflect.Float64:
			return int32(eventFieldVal.Float())
		default:
			slog.Error("unsupported field", "name", progressCfg.GetEventField(), "event", event)
			return 0
		}
	}
	return 1
}

package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
	"math"
	"reflect"
)

var (
	ProgressUpdateFn ProgressUpdateFunc = DefaultProgressUpdater // 进度更新接口
	ProgressInitFn   ProgressUpdateFunc                          // 进度初始化接口
)

// 进度读取接口
type ProgressHolder interface {
	GetProgress() int32
	SetProgress(progress int32)
}

// 进度更新接口
// 返回值: 进度值的实际增加值
// example:
//
//	进度: 抽卡100次
//	事件: 抽卡(5连抽)
//	进度+5
type ProgressUpdateFunc func(progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) int32

// 初始化进度,更新初始进度
// 返回值: true表示有进度更新
//
//	examples:
//	任务举例:玩家升级到10级,当5级玩家接任务时,初始进度就是5/10
func InitProgress(progressHolder ProgressHolder, arg any, progressCfg *pb.ProgressCfg) bool {
	if ProgressInitFn != nil {
		return ProgressInitFn(progressHolder, arg, progressCfg) > 0
	}
	return false
}

// 检查事件是否触发进度的更新,并更新进度
func UpdateProgress(progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) bool {
	if ProgressUpdateFn != nil {
		return ProgressUpdateFn(progressHolder, event, progressCfg) > 0
	}
	return false
}

// 默认的进度更新接口
func DefaultProgressUpdater(progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) int32 {
	// 通用事件匹配,先检查事件名是否匹配
	if progressCfg.GetType() == int32(pb.ProgressType_ProgressType_Event) {
		eventTyp := reflect.TypeOf(event)
		if eventTyp.Kind() == reflect.Pointer {
			eventTyp = eventTyp.Elem()
		}
		if eventTyp.Name() != progressCfg.GetEvent() {
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
			slog.Error("unsupported field", "progressCfg", progressCfg, "fieldName", fieldName, "event", event)
			return 0
		}
	}
	// 检查progressCfg.EventField
	progress := int32(1)
	// 如果配置了事件属性字段,则读取该字段的值作为进度值,没配置就默认进度值是1
	if progressCfg.GetEventField() != "" {
		eventFieldVal := eventVal.FieldByName(progressCfg.GetEventField())
		if !eventFieldVal.IsValid() {
			// event没有这个属性
			slog.Error("unsupported EventField", "progressCfg", progressCfg, "event", event)
			return 0
		}
		switch eventFieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			progress = int32(eventFieldVal.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			progress = int32(eventFieldVal.Uint())
		case reflect.Float32, reflect.Float64:
			progress = int32(eventFieldVal.Float())
		default:
			slog.Error("unsupported EventField type", "progressCfg", progressCfg, "event", event)
			return 0
		}
	}
	return CheckAndSetProgress(progressHolder, progressCfg, progress)
}

func CheckAndSetProgress(progressHolder ProgressHolder, progressCfg *pb.ProgressCfg, progress int32) int32 {
	if progressHolder.GetProgress()+progress > progressCfg.GetTotal() {
		progress = progressCfg.GetTotal() - progressHolder.GetProgress()
	}
	if progress < 0 {
		progress = 0
	}
	// Q:有进度值减少的需求吗?
	if progress > 0 {
		progressHolder.SetProgress(progressHolder.GetProgress() + progress)
	}
	return progress
}

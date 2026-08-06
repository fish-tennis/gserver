package internal

import (
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
	"math"
	"reflect"
	"sync"
)

// progressFieldKey 用于缓存事件字段的反射索引
type progressFieldKey struct {
	eventType reflect.Type
	fieldName string
}

// 事件字段索引缓存,避免每次 DefaultProgressUpdater 都调用 FieldByName 做线性查找
var _progressFieldIndexCache sync.Map // map[progressFieldKey][]int

// getCachedFieldIndex 获取事件结构体中指定字段的索引,带缓存
func getCachedFieldIndex(eventType reflect.Type, fieldName string) ([]int, bool) {
	key := progressFieldKey{eventType, fieldName}
	if cached, ok := _progressFieldIndexCache.Load(key); ok {
		return cached.([]int), true
	}
	field, ok := eventType.FieldByName(fieldName)
	if !ok {
		return nil, false
	}
	_progressFieldIndexCache.Store(key, field.Index)
	return field.Index, true
}

const (
	PropertyKey = "Property"
)

var (
	ProgressUpdateFn ProgressUpdateFunc = DefaultProgressUpdater           // 进度更新接口
	ProgressInitFn   ProgressInitFunc   = DefaultPropertyInt32InitProgress // 进度初始化接口
)

// 进度值读写接口
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
type ProgressUpdateFunc func(obj any, progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) int32

// 进度初始化接口
type ProgressInitFunc func(obj any, progressHolder ProgressHolder, progressCfg *pb.ProgressCfg) int32

// 初始化进度,更新初始进度
// 返回值: true表示有进度更新
//
//	examples:
//	任务举例:玩家升级到10级,当5级玩家接任务时,初始进度就是5/10
func InitProgress(obj any, progressHolder ProgressHolder, progressCfg *pb.ProgressCfg) bool {
	if ProgressInitFn != nil {
		return ProgressInitFn(obj, progressHolder, progressCfg) > 0
	}
	return false
}

// 检查事件是否触发进度的更新,并更新进度
func UpdateProgress(obj any, progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) bool {
	if ProgressUpdateFn != nil && progressCfg != nil {
		return ProgressUpdateFn(obj, progressHolder, event, progressCfg) > 0
	}
	return false
}

// 默认的进度更新接口
func DefaultProgressUpdater(obj any, progressHolder ProgressHolder, event any, progressCfg *pb.ProgressCfg) int32 {
	// 通用事件匹配,先检查事件名是否匹配
	eventTyp := reflect.TypeOf(event)
	isPointer := eventTyp.Kind() == reflect.Pointer
	if isPointer {
		eventTyp = eventTyp.Elem()
	}
	if progressCfg.GetType() == int32(pb.ProgressType_ProgressType_Event) {
		if eventTyp.Name() != progressCfg.GetEvent() {
			return 0
		}
	}
	var eventVal reflect.Value
	if isPointer {
		eventVal = reflect.ValueOf(event).Elem()
	} else {
		eventVal = reflect.ValueOf(event)
	}
	// 事件字段值(字符串形式),这里也包含了对比较操作符为=的数值字段值的支持
	for fieldName, fieldValue := range progressCfg.GetStringEventFields() {
		fieldIndex, ok := getCachedFieldIndex(eventTyp, fieldName)
		if !ok {
			return 0
		}
		eventFieldVal := eventVal.FieldByIndex(fieldIndex)
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
	// 数值类型的事件字段值(只支持整数和bool 数值字段支持更丰富的Op操作符)
	for fieldName, fieldValueCompareCfg := range progressCfg.GetIntEventFields() {
		fieldIndex, ok := getCachedFieldIndex(eventTyp, fieldName)
		if !ok {
			return 0
		}
		eventFieldVal := eventVal.FieldByIndex(fieldIndex)
		switch eventFieldVal.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			eventFieldInt := eventFieldVal.Int()
			if !CompareOpValue(obj, int32(eventFieldInt), fieldValueCompareCfg) {
				return 0
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			eventFieldInt := eventFieldVal.Uint()
			if !CompareOpValue(obj, int32(eventFieldInt), fieldValueCompareCfg) {
				return 0
			}
		case reflect.Bool:
			eventFieldBool := eventFieldVal.Bool()
			eventFieldInt := 0
			if eventFieldBool {
				eventFieldInt = 1
			}
			if !CompareOpValue(obj, int32(eventFieldInt), fieldValueCompareCfg) {
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
	if progressCfg.GetProgressField() != "" {
		fieldIndex, ok := getCachedFieldIndex(eventTyp, progressCfg.GetProgressField())
		if !ok {
			// event没有这个属性
			slog.Error("unsupported EventField", "progressCfg", progressCfg, "event", event)
			return 0
		}
		eventFieldVal := eventVal.FieldByIndex(fieldIndex)
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

// PropertyInt32的实现类的进度值初始化接口
func DefaultPropertyInt32InitProgress(obj any, progressHolder ProgressHolder, progressCfg *pb.ProgressCfg) int32 {
	if propertyGetter, ok := obj.(PropertyInt32); ok {
		if propertyName, ok2 := progressCfg.StringEventFields[PropertyKey]; ok2 {
			slog.Debug("DefaultPropertyInt32InitProgress", "name", propertyName, "value", propertyGetter.GetPropertyInt32(propertyName, nil))
			return CheckAndSetProgress(progressHolder, progressCfg, propertyGetter.GetPropertyInt32(propertyName, nil))
		}
	}
	return 0
}

// 设置进度值,返回实际增加的进度值,不会超出ProgressCfg.Total
// 比如某个任务当前进度是8/10,progressIncValue是5,CheckAndSetProgress后进度变成10/10,返回值是2
func CheckAndSetProgress(progressHolder ProgressHolder, progressCfg *pb.ProgressCfg, progressIncValue int32) int32 {
	// 用 int64 做加法判断,防止两个正值溢出为负数绕过封顶逻辑
	sum := int64(progressHolder.GetProgress()) + int64(progressIncValue)
	if sum > int64(progressCfg.GetTotal()) {
		progressIncValue = progressCfg.GetTotal() - progressHolder.GetProgress()
	}
	if progressIncValue < 0 {
		progressIncValue = 0
	}
	if progressIncValue > 0 {
		progressHolder.SetProgress(progressHolder.GetProgress() + progressIncValue)
	}
	return progressIncValue
}

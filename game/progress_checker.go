package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 注册进度接口
func RegisterProgressCheckers() *internal.ProgressMgr {
	progressMgr := internal.NewProgressMgr()

	progressMgr.RegisterDefault(int32(pb.ProgressType_ProgressType_Event),
		&pb.EventFight{},
	)

	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_PlayerProperty), &pb.EventPlayerProperty{},
		onPlayerPropertyUpdate, onPlayerPropertyInit)
	progressMgr.RegisterWithInit(int32(pb.ProgressType_ProgressType_ActivityProperty), &pb.EventActivityProperty{},
		onActivityPropertyUpdate, onActivityPropertyInit)
	return progressMgr
}

func parsePlayer(arg any) *Player {
	switch v := arg.(type) {
	case *Player:
		return v
	case *ActivityDefault:
		return v.Activities.player
	default:
		slog.Error("parsePlayerErr", "arg", arg)
		return nil
	}
}

func parseActivity(arg any) internal.Activity {
	switch v := arg.(type) {
	case *ActivityDefault:
		return v
	default:
		return nil
	}
}

// 玩家属性值对应的进度初始化
func onPlayerPropertyInit(arg any, progressCfg *pb.ProgressCfg) int32 {
	// 当前属性值
	propertyName := progressCfg.Properties["PropertyName"]
	player := parsePlayer(arg)
	if player == nil {
		return 0
	}
	slog.Debug("onPlayerPropertyInit", "name", propertyName, "value", player.GetPropertyInt32(propertyName))
	return player.GetPropertyInt32(propertyName)
}

// 玩家属性值变化对应的进度更新
func onPlayerPropertyUpdate(event any, progressCfg *pb.ProgressCfg) int32 {
	propertyName := progressCfg.Properties["PropertyName"]
	eventPropertyInc, ok := event.(*pb.EventPlayerProperty)
	if !ok {
		slog.Error("onPlayerPropertyUpdateErr", "event", event, "progressCfg", progressCfg)
		return 0
	}
	if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
		slog.Debug("onPlayerPropertyUpdate", "name", propertyName, "value", eventPropertyInc.GetPropertyValue())
		return eventPropertyInc.GetPropertyValue()
	}
	return 0
}

// 活动属性值对应的进度初始化
func onActivityPropertyInit(arg any, progressCfg *pb.ProgressCfg) int32 {
	// 当前属性值
	propertyName := progressCfg.Properties["PropertyName"]
	activity := parseActivity(arg)
	if activity == nil {
		return 0
	}
	slog.Debug("onActivityPropertyInit", "name", propertyName, "value", activity.GetPropertyInt32(propertyName))
	return activity.GetPropertyInt32(propertyName)
}

// 活动属性值变化对应的进度更新
func onActivityPropertyUpdate(event any, progressCfg *pb.ProgressCfg) int32 {
	propertyName := progressCfg.Properties["PropertyName"]
	eventPropertyInc, ok := event.(*pb.EventActivityProperty)
	if !ok {
		slog.Error("onActivityPropertyUpdateErr", "event", event, "progressCfg", progressCfg)
		return 0
	}
	if propertyName != "" && eventPropertyInc.GetPropertyName() == propertyName {
		slog.Debug("onActivityPropertyUpdate", "name", propertyName, "value", eventPropertyInc.GetPropertyValue())
		return eventPropertyInc.GetPropertyValue()
	}
	return 0
}

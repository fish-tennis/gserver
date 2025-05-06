package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

const (
	PropertyKey = "Property"
)

// 注册进度更新和初始化接口
func init() {
	internal.ProgressUpdateFn = internal.DefaultProgressUpdater
	internal.ProgressInitFn = DefaultInitProgress
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

func DefaultInitProgress(progressHolder internal.ProgressHolder, arg any, progressCfg *pb.ProgressCfg) int32 {
	if progressCfg.GetEvent() == "EventPlayerProperty" {
		propertyName := progressCfg.Properties[PropertyKey]
		player := parsePlayer(arg)
		if player == nil {
			return 0
		}
		slog.Debug("DefaultInitProgress", "name", propertyName, "value", player.GetPropertyInt32(propertyName))
		return internal.CheckAndSetProgress(progressHolder, progressCfg, player.GetPropertyInt32(propertyName))
	} else if progressCfg.GetEvent() == "EventActivityProperty" {
		propertyName := progressCfg.Properties[PropertyKey]
		player := parseActivity(arg)
		if player == nil {
			return 0
		}
		slog.Debug("DefaultInitProgress", "name", propertyName, "value", player.GetPropertyInt32(propertyName))
		return internal.CheckAndSetProgress(progressHolder, progressCfg, player.GetPropertyInt32(propertyName))
	}
	return 0
}

package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

func RegisterConditionCheckers() *internal.ConditionMgr {
	conditionMgr := internal.NewConditionMgr()
	conditionMgr.RegisterWithInit(int32(pb.ConditionType_ConditionType_PlayerLevelup), &pb.EventPlayerLevelup{}, func(event interface{}, conditionCfg *internal.ConditionCfg) int32 {
		// 升级条件,玩家的等级就是进度值
		return event.(*pb.EventPlayerLevelup).Level
	}, func(arg interface{}, conditionCfg *internal.ConditionCfg) int32 {
		// 初始进度是玩家当时的等级
		return arg.(*Player).GetBaseInfo().Data.Level
	})
	conditionMgr.RegisterDefault(int32(pb.ConditionType_ConditionType_Fight), &pb.EventFight{})
	return conditionMgr
}
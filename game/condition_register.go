package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
)

// 注册条件检查接口
func init() {
	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_PlayerPropertyCompare),
		PlayerPropertyInt32Checker)

	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_ActivityPropertyCompare),
		internal.DefaultPropertyInt32Checker)
}

// CheckConditions的obj参数可以传入*Player,PlayerComponent,*ActivityDefault等对象,
// 会自动解析出*Player对象,从而获取玩家的属性值
func ParsePlayer(obj any) *Player {
	switch t := obj.(type) {
	case *Player:
		return t
	case PlayerComponent:
		return t.GetPlayer()
	case *ActivityDefault:
		if t.Activities != nil {
			return t.Activities.GetPlayer()
		}
	}
	return nil
}

// 玩家属性值比较条件检查器
func PlayerPropertyInt32Checker(obj any, conditionCfg *pb.ConditionCfg) bool {
	// obj可能是*Player,PlayerComponent,*ActivityDefault等对象,解析出*Player对象
	// 从而获取玩家的属性值
	player := ParsePlayer(obj)
	if player == nil {
		slog.Error("PlayerPropertyInt32CheckerErr", "obj", obj, "conditionCfg", conditionCfg)
		return false
	}
	return internal.DefaultPropertyInt32Checker(player, conditionCfg)
}

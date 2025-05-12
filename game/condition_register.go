package game

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

// 注册条件检查接口
func init() {
	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_PlayerPropertyCompare),
		internal.DefaultPropertyInt32Checker)

	internal.RegisterConditionChecker(int32(pb.ConditionType_ConditionType_ActivityPropertyCompare),
		internal.DefaultPropertyInt32Checker)
}

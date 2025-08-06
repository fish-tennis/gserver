package cfg

import (
	"github.com/fish-tennis/gserver/pb"
)

var (
	MaxLevel int32
)

func LevelAfterLoad(mgr any, mgrName, messageName, fileName string) {
	levels := mgr.(*DataSlice[*pb.LevelExp])
	MaxLevel = int32(levels.Len())
}

// 下一级所需要经验值
func GetNeedExp(nextLevel int32) int32 {
	return LevelExps.GetCfg(int(nextLevel - 1)).GetNeedExp()
}

package cfg

import (
	"github.com/fish-tennis/gserver/pb"
)

var (
	MaxLevel int32
)

func init() {
	register.LevelExpsProcess = LevelAfterLoad
}

func LevelAfterLoad(mgr *DataSlice[*pb.LevelExp]) error {
	MaxLevel = int32(mgr.Len())
	return nil
}

// 下一级所需要经验值
func GetNeedExp(nextLevel int32) int32 {
	return LevelExps.GetCfg(int(nextLevel - 1)).GetNeedExp()
}

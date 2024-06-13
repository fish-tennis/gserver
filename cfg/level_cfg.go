package cfg

import (
	"github.com/fish-tennis/gserver/pb"
)

var (
	_levelCfgMgr = &LevelCfgMgr{}
)

func init() {
	RegisterCfgLoader(&CfgLoaderOption{
		Loader:   _levelCfgMgr,
		FileName: "levelcfg.csv",
	})
}

// 等级配置数据管理
type LevelCfgMgr struct {
	// 每一级升级所需要的经验
	DataSlice[*pb.LevelExp]
}

// singleton
func GetLevelCfgMgr() *LevelCfgMgr {
	return _levelCfgMgr
}

// 最大等级
func (this *LevelCfgMgr) GetMaxLevel() int32 {
	return int32(this.Len())
}

// 下一级所需要经验值
func (this *LevelCfgMgr) GetNeedExp(nextLevel int32) int32 {
	return this.cfgs[nextLevel-1].NeedExp
}

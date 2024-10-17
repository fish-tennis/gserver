package cfg

import (
	"github.com/fish-tennis/gserver/pb"
)

var (
	_levelCfgLoader = Register(func() any {
		return new(LevelCfgMgr)
	}, First)
)

// 等级配置数据管理
type LevelCfgMgr struct {
	// 每一级升级所需要的经验
	*DataSlice[*pb.LevelExp] `cfg:"levelcfg.csv"`
}

// singleton
func GetLevelCfgMgr() *LevelCfgMgr {
	return _levelCfgLoader.Load().(*LevelCfgMgr)
}

// 最大等级
func (m *LevelCfgMgr) GetMaxLevel() int32 {
	return int32(m.Len())
}

// 下一级所需要经验值
func (m *LevelCfgMgr) GetNeedExp(nextLevel int32) int32 {
	return m.cfgs[nextLevel-1].NeedExp
}

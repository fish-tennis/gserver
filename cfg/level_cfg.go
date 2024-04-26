package cfg

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/tool"
)

var (
	_levelCfgMgr *LevelCfgMgr
)

// 等级配置数据管理
type LevelCfgMgr struct {
	// 每一级升级所需要的经验
	needExps []*pb.LevelExp
}

func GetLevelCfgMgr() *LevelCfgMgr {
	if _levelCfgMgr == nil {
		_levelCfgMgr = &LevelCfgMgr{
			needExps: make([]*pb.LevelExp, 0),
		}
	}
	return _levelCfgMgr
}

func (this *LevelCfgMgr) GetMaxLevel() int32 {
	return int32(len(this.needExps))
}

func (this *LevelCfgMgr) GetNeedExp(nextLevel int32) int32 {
	return this.needExps[nextLevel-1].NeedExp
}

func (this *LevelCfgMgr) Load(fileName string) bool {
	option := &tool.CsvOption{
		DataBeginRowIndex: 1,
	}
	s := make([]*pb.LevelExp, 0)
	var err error
	this.needExps, err = tool.ReadCsvFileSlice(fileName, s, option)
	if err != nil {
		logger.Error("csv read err:%v", err)
		return false
	}
	return true
}

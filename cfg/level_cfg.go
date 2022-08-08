package cfg

import (
	"encoding/csv"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"os"
)

var (
	_levelCfgMgr *LevelCfgMgr
)

// 等级配置数据管理
type LevelCfgMgr struct {
	// 每一级升级所需要的经验
	needExps []int32
}

func GetLevelCfgMgr() *LevelCfgMgr {
	if _levelCfgMgr == nil {
		_levelCfgMgr = &LevelCfgMgr{
			needExps: make([]int32, 0),
		}
	}
	return _levelCfgMgr
}

func (this *LevelCfgMgr) GetMaxLevel() int32 {
	return int32(len(this.needExps))
}

func (this *LevelCfgMgr) GetNeedExp(nextLevel int32) int32 {
	return this.needExps[nextLevel-1]
}

func (this *LevelCfgMgr) Load(fileName string) bool {
	file, err := os.Open(fileName)
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	defer file.Close()
	reader := csv.NewReader(file)
	data, err := reader.ReadAll()
	if err != nil {
		logger.Error("%v", err)
		return false
	}
	for _, line := range data {
		if len(this.needExps)+1 != util.Atoi(line[0]) {
			logger.Error("level cfg error: %v", line)
		}
		this.needExps = append(this.needExps, int32(util.Atoi(line[1])))
	}
	return false
}

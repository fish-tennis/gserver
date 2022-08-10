package gameplayer

import (
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.BaseInfo `db:"BaseInfo;plain"`
}

func NewBaseInfo(player *Player, data *pb.BaseInfo) *BaseInfo {
	component := &BaseInfo{
		PlayerDataComponent: PlayerDataComponent{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   "BaseInfo",
			},
		},
		Data: &pb.BaseInfo{
			Level: 1,
			Exp: 0,
		},
	}
	if data != nil {
		component.Data = data
	}
	return component
}

func (this *BaseInfo) IncExp(incExp int32) {
	this.Data.Exp += incExp
	for {
		if this.Data.Level < cfg.GetLevelCfgMgr().GetMaxLevel() {
			needExp := cfg.GetLevelCfgMgr().GetNeedExp(this.Data.Level+1)
			if needExp > 0 && this.Data.Exp >= needExp {
				this.Data.Level++
				this.Data.Exp -= needExp
				this.GetPlayer().FireConditionEvent(&pb.EventPlayerLevelup{
					PlayerId: this.GetPlayerId(),
					Level: this.Data.Level,
				})
				continue
			}
		}
		break
	}
	logger.Debug("%v exp:%v lvl:%v", this.GetPlayerId(), this.Data.Exp, this.Data.Level)
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}
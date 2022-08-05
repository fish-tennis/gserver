package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 编译期检查是否实现了Saveable接口
// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ internal.Saveable = (*BaseInfo)(nil)

// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.BaseInfo `db:"baseinfo;plain"`
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
	lvl := this.Data.Exp/100
	if lvl > this.Data.Level {
		this.Data.Level = lvl
		this.GetPlayer().FireConditionEvent(&pb.EventPlayerLevelup{
			PlayerId: this.GetPlayerId(),
			Level: lvl,
		})
	}
	logger.Debug("exp:%v lvl:%v", this.Data.Exp, this.Data.Level)
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}
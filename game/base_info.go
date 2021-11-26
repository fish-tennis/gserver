package game

import (
	"github.com/fish-tennis/gserver/pb"
)

// 玩家基础信息
type BaseInfo struct {
	BaseComponent
	baseInfo *pb.BaseInfo
}

func NewBaseInfo(player *Player, playerData *pb.PlayerData) *BaseInfo {
	if playerData.BaseInfo == nil {
		playerData.BaseInfo = &pb.BaseInfo{
			Name: player.GetName(),
			Level: 1,
			Exp: 0,
		}
	}
	return &BaseInfo{
		BaseComponent: BaseComponent{
			Player: player,
		},
		baseInfo: playerData.GetBaseInfo(),
	}
}

func (this *BaseInfo) GetId() int {
	return 1
}

func (this *BaseInfo) GetName() string {
	return "baseinfo"
}

// 需要保存的数据
func (this *BaseInfo) DbData() interface{} {
	return this.baseInfo
}

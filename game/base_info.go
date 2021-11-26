package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家基础信息组件
type BaseInfo struct {
	BaseComponent
	data *pb.BaseInfo
}

func NewBaseInfo(player *Player, playerData *pb.PlayerData) *BaseInfo {
	var baseInfo *pb.BaseInfo
	if playerData.BaseInfo == nil {
		baseInfo = &pb.BaseInfo{
			Name: player.GetName(),
			Level: 1,
			Exp: 0,
		}
	} else {
		baseInfo = playerData.BaseInfo
	}
	gnet.LogDebug("%v", baseInfo)
	return &BaseInfo{
		BaseComponent: BaseComponent{
			Player: player,
		},
		data: baseInfo,
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
	// 演示明文保存数据
	return this.data
}

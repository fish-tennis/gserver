package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

// 玩家基础信息
type BaseInfo struct {
	*BaseComponent
	baseInfo *pb.BaseInfo
}

func NewBaseInfo(player *Player, playerData *pb.PlayerData) *BaseInfo {
	var baseInfo *pb.BaseInfo
	if len(playerData.BaseInfo) == 0 {
		baseInfo = &pb.BaseInfo{
			Name: player.GetName(),
			Level: 1,
			Exp: 0,
		}
	} else {
		baseInfo = &pb.BaseInfo{}
		proto.Unmarshal(playerData.BaseInfo, baseInfo)
		// test
		baseInfo.Exp = baseInfo.Exp + 1
	}
	gnet.LogDebug("%v", baseInfo)
	return &BaseInfo{
		BaseComponent: &BaseComponent{
			Player: player,
		},
		baseInfo: baseInfo,
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
	data,err := proto.Marshal(this.baseInfo)
	if err != nil {
		gnet.LogError("%v", err)
		return nil
	}
	return data
}

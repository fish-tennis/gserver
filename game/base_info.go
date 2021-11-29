package game

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
)

// 玩家基础信息组件
type BaseInfo struct {
	DataComponent
	data *pb.BaseInfo
}

func NewBaseInfo(player *Player, baseInfo *pb.BaseInfo) *BaseInfo {
	component := &BaseInfo{
		DataComponent: DataComponent{
			BaseComponent:BaseComponent{
				Player: player,
				id: 1,
				name: "baseinfo",
			},
		},
	}
	data := baseInfo
	if data == nil {
		data = &pb.BaseInfo{
			Name: player.GetName(),
			Level: 1,
			Exp: 0,
		}
		component.SetDirty(true)
	}
	gnet.LogDebug("%v", data)
	component.data = data
	component.dataFun = component.DbData
	return component
}

// 需要保存的数据
func (this *BaseInfo) DbData() interface{} {
	// 演示明文保存数据
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.data
}

package game

import (
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
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
				name: "BaseInfo",
			},
		},
	}
	data := baseInfo
	if data == nil {
		data = &pb.BaseInfo{
			Level: 1,
			Exp: 0,
		}
		component.SetDirty()
	}
	component.data = data
	logger.Debug("%v level:%v exp:%v",component.GetPlayerId(), component.data.Level, component.data.Exp)
	return component
}

// 需要保存的数据
func (this *BaseInfo) Serialize(forCache bool) interface{} {
	if forCache {
		data,err := proto.Marshal(this.data)
		if err != nil {
			logger.Error("%v", err)
			return nil
		}
		return data
	}
	// 演示明文保存数据
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.data
}

func (this *BaseInfo) Deserialize(bytes []byte) error {
	data := &pb.BaseInfo{}
	proto.Unmarshal(bytes, data)
	this.data = data
	return nil
}

func (this *BaseInfo) IncExp(incExp int32) {
	this.data.Exp += incExp
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}
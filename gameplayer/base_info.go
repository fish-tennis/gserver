package gameplayer

import (
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
)

// 编译期检查是否实现了Saveable接口
// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ internal.Saveable = (*BaseInfo)(nil)

// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	data *pb.BaseInfo
}

func NewBaseInfo(player *Player, data *pb.BaseInfo) *BaseInfo {
	component := &BaseInfo{
		PlayerDataComponent: PlayerDataComponent{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   "BaseInfo",
			},
		},
		data: &pb.BaseInfo{
			Level: 1,
			Exp: 0,
		},
	}
	if data != nil {
		component.data = data
	}
	return component
}

func (this *BaseInfo) DbData() (dbData interface{}, protoMarshal bool) {
	// 演示明文保存数据库
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.data,false
}

func (this *BaseInfo) CacheData() interface{} {
	return this.data
}

func (this *BaseInfo) IncExp(incExp int32) {
	this.data.Exp += incExp
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}
package player

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
	DataComponent
	data *pb.BaseInfo
}

func NewBaseInfo(player *Player) *BaseInfo {
	component := &BaseInfo{
		DataComponent: DataComponent{
			BaseComponent: BaseComponent{
				Player: player,
				Name:   "BaseInfo",
			},
		},
		data: &pb.BaseInfo{
			Level: 1,
			Exp: 0,
		},
	}
	return component
}

// 需要保存的数据
func (this *BaseInfo) Save(forCache bool) (saveData interface{}, isPlain bool) {
	if forCache {
		// 保存到缓存时,进行序列化
		return this.data,false
	}
	// 演示明文保存数据
	// 优点:便于查看,数据库语言可直接操作字段
	// 缺点:字段名也会保存到数据库,占用空间多
	return this.data,true
}

func (this *BaseInfo) Load(data interface{}) error {
	switch t := data.(type) {
	case *pb.BaseInfo:
		// 加载明文数据
		this.data = t
		logger.Debug("%v", this.data)
	case []byte:
		// 反序列化
		err := internal.LoadWithProto(data, this.data)
		logger.Debug("%v", this.data)
		return err
	}
	return nil
}

func (this *BaseInfo) IncExp(incExp int32) {
	this.data.Exp += incExp
	// 修改了需要保存的数据后,必须设置标记
	this.SetDirty()
}
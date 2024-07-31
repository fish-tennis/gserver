package game

import (
	"github.com/fish-tennis/gentity"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

const (
	// 组件名
	ComponentNameProcessStatInfo = "ProcessStatInfo"
)

// 利用go的init进行组件的自动注册
func init() {
	_globalEntityComponentRegister.Register(ComponentNameProcessStatInfo, 0, func(globalEntity *GlobalEntity, _ any) gentity.Component {
		return &ProcessStatInfo{
			BaseComponent: gentity.NewBaseComponent(globalEntity, ComponentNameProcessStatInfo),
			StatInfo:      gentity.NewProtoData(&pb.ProcessStatInfo{}),
		}
	})
}

// 进程统计信息组件
type ProcessStatInfo struct {
	*gentity.BaseComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	StatInfo *gentity.ProtoData[*pb.ProcessStatInfo] `db:"plain"`
}

func (this *GlobalEntity) GetProcessStatInfo() *ProcessStatInfo {
	return this.GetComponentByName(ComponentNameProcessStatInfo).(*ProcessStatInfo)
}

func (this *ProcessStatInfo) HandleStartupReq(cmd PacketCommand, req *pb.StartupReq) {
	this.StatInfo.Data.LastStartupTimestamp = req.Timestamp
	this.StatInfo.SetDirty()
	logger.Debug("HandleStartupReq")
}

func (this *ProcessStatInfo) HandleShutdownReq(cmd PacketCommand, req *pb.ShutdownReq) {
	this.StatInfo.Data.LastShutdownTimestamp = req.Timestamp
	this.StatInfo.SetDirty()
	logger.Debug("HandleShutdownReq")
}

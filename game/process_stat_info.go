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
	RegisterGlobalEntityComponentCtor(ComponentNameProcessStatInfo, 0, func(globalEntity *GlobalEntity, loadData *pb.GlobalEntityData) gentity.Component {
		component := &ProcessStatInfo{
			DataComponent: *gentity.NewDataComponent(globalEntity, ComponentNameProcessStatInfo),
			Data:          &pb.ProcessStatInfo{},
		}
		gentity.LoadData(component, loadData.GetProcessStatInfo())
		return component
	})
}

// 进程统计信息组件
type ProcessStatInfo struct {
	gentity.DataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.ProcessStatInfo `db:"ProcessStatInfo;plain"`
}

func (this *GlobalEntity) GetProcessStatInfo() *ProcessStatInfo {
	return this.GetComponentByName(ComponentNameProcessStatInfo).(*ProcessStatInfo)
}

func (this *ProcessStatInfo) HandleStartupReq(cmd PacketCommand, req *pb.StartupReq) {
	this.Data.LastStartupTimestamp = req.Timestamp
	this.SetDirty()
	logger.Debug("HandleStartupReq")
}

func (this *ProcessStatInfo) HandleShutdownReq(cmd PacketCommand, req *pb.ShutdownReq) {
	this.Data.LastShutdownTimestamp = req.Timestamp
	this.SetDirty()
	logger.Debug("HandleShutdownReq")
}

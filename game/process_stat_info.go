package game

import (
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
)

// 进程统计信息组件
type ProcessStatInfo struct {
	gentity.DataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.ProcessStatInfo `db:"ProcessStatInfo;plain"`
}

func NewProcessStatInfo(globalEntity *GlobalEntity, data *pb.ProcessStatInfo) *ProcessStatInfo {
	component := &ProcessStatInfo{
		DataComponent: *gentity.NewDataComponent(globalEntity, "ProcessStatInfo"),
		Data:          &pb.ProcessStatInfo{},
	}
	if data != nil {
		component.Data = data
	}
	return component
}

func (this *ProcessStatInfo) OnStartup() {
	this.Data.LastStartupTimestamp = GetGlobalEntity().GetTimerEntries().Now().Unix()
	this.SetDirty()
}

func (this *ProcessStatInfo) OnShutdown() {
	this.Data.LastShutdownTimestamp = GetGlobalEntity().GetTimerEntries().Now().Unix()
	this.SetDirty()
}

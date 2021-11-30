package common

import (
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cache"
	"os"
	"os/signal"
	"syscall"
)

type Server interface {
	Init() bool
	Run()
	OnExit()
}

// 服务器基础流程
type BaseServer struct {
}

// 加载配置,网络初始化等
func (this *BaseServer) Init() bool {
	gnet.LogDebug("BaseServer.Init")
	return true
}

// 运行
func (this *BaseServer) Run() {
	gnet.LogDebug("BaseServer.Run")
}

func (this *BaseServer) OnExit() {
	gnet.LogDebug("BaseServer.OnExit")
}

// 等待系统关闭信号
func (this *BaseServer) WaitExit() {
	gnet.LogDebug("BaseServer.WaitExit")
	killSignalChan := make(chan os.Signal, 1)
	signal.Notify(killSignalChan, os.Interrupt, os.Kill, syscall.SIGTERM)
	// TODO: windows系统上,加一个控制台输入,已方便调试
	select {
	case <-killSignalChan:
		break
	}
	this.OnExit()
	if cache.GetRedis() != nil {
		cache.GetRedis().Close()
	}
	gnet.LogDebug("Exit")
}

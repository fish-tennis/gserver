package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameserver"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/loginserver"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	isDaemon := false
	configFile := ""
	// 配置文件名格式: serverType_serverId.json
	flag.StringVar(&configFile, "conf", "", "server's config file")
	flag.BoolVar(&isDaemon, "d", false, "daemon mode")
	flag.Parse()

	if isDaemon {
		daemon()
		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	gnet.SetLogLevel(gnet.DebugLevel)
	gentity.SetLogger(gnet.GetLogger())
	rand.Seed(time.Now().UnixNano())

	// 根据命令行参数 创建不同的服务器实例
	serverType := getServerTypeFromConfigFile(configFile)
	server := createServer(serverType)
	internal.SetServer(server)

	// context实现优雅的协程关闭通知
	ctx,cancel := context.WithCancel(context.Background())
	// 服务器初始化
	if !server.Init(ctx, configFile) {
		panic("server init error")
	}
	// 服务器运行
	server.Run(ctx)

	// 监听系统的kill信号
	signalKillNotify := make(chan os.Signal, 1)
	signal.Notify(signalKillNotify, os.Interrupt, os.Kill, syscall.SIGTERM)
	if !isDaemon {
		// 加一个控制台输入,以方便调试
		go func() {
			consoleReader := bufio.NewReader(os.Stdin)
			for {
				lineBytes, _, _ := consoleReader.ReadLine()
				line := strings.ToLower(string(lineBytes))
				logger.Info("line:%v", line)
				if line == "close" || line == "exit" {
					logger.Info("kill by console input")
					// 模拟一个kill信号,以方便测试服务器退出流程
					signalKillNotify <- os.Kill
					return
				}
			}
		}()
	}
	// 阻塞等待系统关闭信号
	logger.Info("wait for kill signal")
	select {
	case <-signalKillNotify:
		logger.Info("signalKillNotify, cancel ctx")
		// 通知所有协程关闭,所有监听<-ctx.Done()的地方会收到通知
		cancel()
		break
	}
	// 清理
	server.Exit()
}

func daemon() {
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-d=true" {
			args[i] = "-d=false"
			break
		}
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Start()
	fmt.Println("[PID]", cmd.Process.Pid)
	os.Exit(0)
}

// 从配置文件名解析出服务器类型
func getServerTypeFromConfigFile(configFile string) string {
	baseFileName := filepath.Base(configFile) // login_1.json
	idx := strings.Index(baseFileName, "_")
	return baseFileName[0:idx]
}

// 创建相应类型的服务器
func createServer(serverType string) internal.Server {
	switch serverType {
	case "login":
		return new(loginserver.LoginServer)
	case "game":
		return new(gameserver.GameServer)
	}
	panic("err server type")
}

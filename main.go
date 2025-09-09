package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameserver"
	"github.com/fish-tennis/gserver/gateserver"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/loginserver"
	"gopkg.in/natefinch/lumberjack.v2"
	"log/slog"
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
	cfgDir := ""
	// 配置文件名格式: serverType_xxx.json
	flag.StringVar(&configFile, "conf", "", "server's config file")
	flag.StringVar(&cfgDir, "cfg", "cfgdata", "cfg dir")
	flag.BoolVar(&isDaemon, "d", false, "daemon mode")
	flag.Parse()

	if isDaemon {
		daemon()
		return
	}

	runtime.GOMAXPROCS(runtime.NumCPU())
	rand.Seed(time.Now().UnixNano())

	// 根据命令行参数 创建不同的服务器实例
	baseFileName := filepath.Base(configFile)                                   // login_test.yaml
	baseFileName = strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName)) // login_test
	serverType := getServerTypeFromConfigFile(configFile)                       // login
	initLog(baseFileName, !isDaemon)
	ctx, cancel := context.WithCancel(context.Background())
	server := createServer(ctx, serverType, configFile, cfgDir)
	gentity.SetApplication(server)
	// 服务器初始化
	if !server.Init(ctx, configFile) {
		panic("server init error")
	}
	// 服务器运行
	server.Run(ctx)

	// 监听系统的kill信号
	signalKillNotify := make(chan os.Signal, 1)
	signal.Notify(signalKillNotify, os.Interrupt, os.Kill, syscall.SIGTERM)
	if !isDaemon && runtime.GOOS == "windows" {
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
	if os.Getppid() == 1 {
		return
	}
	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		if args[i] == "-d=true" {
			args[i] = "-d=false"
			break
		}
	}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Start()
	fmt.Println(fmt.Sprintf("%v %v", os.Args[0], args))
	fmt.Println("[PID]", cmd.Process.Pid)
	//os.Exit(0)
}

func initLog(logFileName string, useStdOutput bool) {
	gnet.SetLogger(logger.GetLogger(), gnet.DebugLevel)
	gentity.SetLogger(logger.GetLogger(), gnet.DebugLevel)

	os.Mkdir("log", 0750)
	// 日志轮转与切割
	fileLogger := &lumberjack.Logger{
		Filename:   fmt.Sprintf("log/%v.log", logFileName),
		MaxSize:    10,
		MaxBackups: 100,
		MaxAge:     7,
		Compress:   false,
		LocalTime:  true,
	}
	// 建议使用slog
	debugLevel := &slog.LevelVar{}
	debugLevel.Set(slog.LevelDebug)
	slog.SetDefault(slog.New(logger.NewJsonHandlerWithStdOutput(fileLogger, &slog.HandlerOptions{
		AddSource: true,
		Level:     debugLevel,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.SourceKey {
				source := a.Value.Any().(*slog.Source)
				source.Function = ""
				source.File = logger.GetShortFileName(source.File)
			}
			return a
		},
	}, useStdOutput)))
}

// 从配置文件名解析出服务器类型
func getServerTypeFromConfigFile(configFile string) string {
	baseFileName := filepath.Base(configFile)
	idx := strings.Index(baseFileName, "_") // login_test.yaml
	if idx > 0 {
		return baseFileName[0:idx]
	}
	idx = strings.Index(baseFileName, ".") // login.yaml
	return baseFileName[0:idx]
}

// 创建相应类型的服务器
func createServer(ctx context.Context, serverType, configFile, cfgDir string) gentity.Application {
	switch strings.ToLower(serverType) {
	case strings.ToLower(internal.ServerType_Gate):
		return gateserver.NewGateServer(ctx, configFile, cfgDir)
	case strings.ToLower(internal.ServerType_Login):
		return loginserver.NewLoginServer(ctx, configFile, cfgDir)
	case strings.ToLower(internal.ServerType_Game):
		return gameserver.NewGameServer(ctx, configFile, cfgDir)
	}
	panic("err server type")
}

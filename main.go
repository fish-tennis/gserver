package main

import (
	"flag"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/login"
	"path/filepath"
	"strings"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	// 配置文件名格式: serverType_serverId.json
	configFile := flag.String("config", "", "server's config file")
	flag.Parse()

	// 根据命令行参数 创建不同的服务器实例
	serverType := getServerTypeFromConfigFile(*configFile)
	server := createServer(serverType)
	if !server.Init(*configFile) {
		panic("server init error")
	}
	server.Run()
}

// 从配置文件名解析出服务器类型
func getServerTypeFromConfigFile(configFile string) string {
	baseFileName := filepath.Base(configFile) // login_1.json
	idx := strings.Index(baseFileName, "_")
	return baseFileName[0:idx]
}

// 创建相应类型的服务器
func createServer(serverType string) common.Server {
	switch serverType {
	case "login":
		return new(login.LoginServer)
	case "game":
		return new(game.GameServer)
	}
	panic("err server type")
}

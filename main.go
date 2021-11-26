package main

import (
	"flag"
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/game"
	"github.com/fish-tennis/gserver/login"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	serverType := flag.String("type", "", "server type")
	flag.Parse()

	server := GetServer(*serverType)
	if !server.Init() {
		panic("server init error")
	}
	server.Run()
}

func GetServer(serverType string) common.Server {
	switch serverType {
	case "login":
		server := &login.LoginServer{
		}
		return server
	case "game":
		server := &game.GameServer{
		}
		return server
	}
	panic("err server type")
}

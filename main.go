package main

import (
	"github.com/fish-tennis/gserver/common"
	"github.com/fish-tennis/gserver/login"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			panic(err)
		}
	}()

	server := GetServer()
	if !server.Init() {
		panic("server init error")
	}
	server.Run()
}

func GetServer() common.Server {
	server := &login.LoginServer{
	}
	return server
}

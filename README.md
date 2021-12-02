# gserver
使用[gnet](https://github.com/fish-tennis/gnet)开发的分布式游戏服务器框架

## 设计思路
- 玩家数据的存储接口可替换(mongodb,mysql,redis)
- 使用redis做缓存服务
- 使用protobuf做通讯协议
- 分布式游戏服务器
- 采用Entity-Component,尽可能使模块解耦
- 工具生成辅助代码,提供更友好的调用接口
- 能应用于商业项目

## 编译
项目使用go.mod

由于墙的问题,可以设置 GOPROXY=https://goproxy.cn

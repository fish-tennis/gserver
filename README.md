# gserver
使用[gnet](https://github.com/fish-tennis/gnet)开发的分布式游戏服务器框架

## 设计思路
- 网络库使用[gnet](https://github.com/fish-tennis/gnet)
- 玩家数据的存储接口可替换(mongodb,mysql,redis)
- 使用redis做分布式缓存服务
- 使用protobuf做通讯协议和数据序列化
- 分布式游戏服务器,默认使用redis实现服务注册和发现
- 采用Entity-Component模式,尽可能使模块解耦
- 工具生成辅助代码,提供更友好的调用接口
- 能应用于商业项目

## 功能
- 服务器自动组网
- 游戏服负载均衡,可灵活扩缩容
- 一个账号同时只能登录一个服务器(数据一致性前提)
- 玩家数据修改即时缓存(redis),下线才保存到数据库(mongodb)
- 游戏服宕机后重启,自动修复缓存数据,防止玩家数据回档
- 自动消息注册
- 玩家Entity-Component设计
- 玩家组件事件分发

## 编译
项目使用go.mod

由于墙的问题,可以设置 GOPROXY=https://goproxy.cn

## 编码规范参考
https://github.com/uber-go/guide(https://github.com/uber-go/guide)
https://github.com/xxjwxc/uber_go_guide_cn(https://github.com/xxjwxc/uber_go_guide_cn)

## 使用建议
由于游戏需求多变,且项目类型很多,不同类型的游戏项目差异也很大,设计出一个通用的游戏服务器框架是非常困难的.
因此gserver主要目的是演示如何搭建一个分布式游戏服务器框架,或者说是提供一个模板,实际项目可以在此基础上
进行自己的扩展和修改,无需拘泥于gserver.
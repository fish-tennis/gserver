# gserver
使用[gnet](https://github.com/fish-tennis/gnet)开发的分布式游戏服务器框架

## 设计思路
- 网络库使用[gnet](https://github.com/fish-tennis/gnet)
- 玩家数据的存储接口可替换(mongodb,mysql,redis)
- 分布式缓存
- 使用protobuf做通讯协议和数据序列化
- 服务注册和发现
- 采用Entity-Component模式,尽可能使模块解耦
- 数据绑定,使业务逻辑和数据库操作解耦
- 工具生成辅助代码,提供更友好的调用接口

## 演示功能
- 服务器自动组网
- 游戏服负载均衡
- 一个账号同时只能登录一个服务器(数据一致性前提)
- 玩家数据修改即时缓存,下线才保存到数据库
- 游戏服宕机后重启,自动修复缓存数据,防止玩家数据回档
- 工具生成消息注册和发送消息代码[proto_code_gen](https://github.com/fish-tennis/proto_code_gen)
- 通过反射自动注册消息回调
- 采用Entity-Component设计,模块解耦
- 玩家组件事件分发
- 业务层和数据层分离,业务代码无需操作数据库和缓存
- 通过公会功能演示如何开发分布式的功能
- 通过公会功能演示服务器动态扩缩容的处理方式

## 编译
项目使用go.mod

## 编码规范参考
[https://github.com/uber-go/guide](https://github.com/uber-go/guide)

[https://github.com/xxjwxc/uber_go_guide_cn](https://github.com/xxjwxc/uber_go_guide_cn)

## 使用建议
由于游戏需求多变,且项目类型很多,不同类型的游戏项目差异也很大,设计出一个通用的游戏服务器框架是非常困难的.
因此gserver主要目的是演示如何搭建一个分布式游戏服务器框架,或者说是提供一个模板,实际项目可以在此基础上
进行自己的扩展和修改,无需拘泥于gserver.

## TODO
- 使用struct tag简化数据绑定
- 全局对象的数据绑定

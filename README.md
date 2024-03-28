# gserver
分布式游戏服务器框架

## 设计思路
- 网络库使用[gnet](https://github.com/fish-tennis/gnet)
- 玩家数据的存储接口可替换(mongodb,mysql,redis)
- 使用protobuf做通讯协议和数据序列化
- 采用Entity-Component模式,尽可能使模块解耦
- 数据绑定,使业务逻辑和数据库操作解耦
- 工具生成辅助代码,提供更友好的调用接口

## 演示功能
- 服务器自动组网
- 游戏服负载均衡
- 客户端直连模式和网关模式可选,网关模式支持WebSocket
- 一个账号同时只能登录一个服务器(数据一致性前提)
- 游戏服宕机后重启,自动修复缓存数据,防止玩家数据回档
- 工具生成消息注册和发送消息代码[proto_code_gen](https://github.com/fish-tennis/proto_code_gen)
- 通过反射自动注册消息回调
- 采用Entity-Component设计,模块解耦
- 玩家组件事件分发
- 业务层和数据层分离,业务代码无需操作数据库和缓存
- 通过公会功能演示如何开发分布式的功能
- 通过公会功能演示服务器动态扩缩容的处理方式
- 活动模块,演示了如何保存动态的数据,以及利用json,proto,gentity设计一个灵活扩展的模块

## 数据方案
玩家数据落地使用mongodb(支持扩展为mysql),玩家上线时,从mongodb拉取玩家数据,玩家下线时,把玩家数据保存到mongodb

缓存使用redis,玩家在线期间修改的数据,即时保存到redis,防止服务器crash导致数据丢失

gserver提供了数据绑定的方案,业务层只需要标记哪些数据需要保存,无需自己写代码操作数据库

## 数据绑定
使用go的struct tag,设置对象组件的字段,框架接口会自动对这些字段进行数据库读取保存和缓存更新,极大的简化了业务代码对数据库和缓存的操作

设置组件保存数据
```go
// 玩家的一个组件
type Money struct {
	PlayerDataComponent
	// 该字段必须导出(首字母大写)
	// 使用struct tag来标记该字段需要存数据库,可以设置存储字段名(proto格式存mongo时,使用全小写格式)
	Data *pb.Money `db:"money"`
}
//调用player.SaveDb()会自动把Money.Data保存到mongo,保存时会自动进行proto序列化
//调用player.SaveCache()会自动把Money.Data缓存到redis,保存时会自动进行proto序列化
```

支持明文方式保存数据
```go
// 玩家基础信息组件
type BaseInfo struct {
	PlayerDataComponent
	// plain表示明文存储,在保存到mongo时,不会进行proto序列化
	Data *pb.BaseInfo `db:"baseinfo;plain"`
}
```

支持组合模式
```go
// 玩家的任务组件
type Quest struct {
	BasePlayerComponent
	// 保存数据的子模块:已完成的任务
	Finished *FinishedQuests `child:"finished"`
	// 保存数据的子模块:当前任务列表
	Quests *CurQuests `child:"quests"`
}
// 已完成的任务
type FinishedQuests struct {
    BaseDirtyMark
    Finished []int32 `db:"finished;plain"`
}
// 当前任务列表
type CurQuests struct {
    BaseMapDirtyMark
    Quests map[int32]*pb.QuestData `db:"quests"`
}
```

## 消息回调
支持自动注册消息回调
```go
// 客户端发给服务器的完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (this *Quest) OnFinishQuestReq(reqCmd gnet.PacketCommand, req *pb.FinishQuestReq) {
	// logic code ...
}
```
```go
// 这种格式写的函数可以自动注册非客户端的消息回调
func (this *BaseInfo) HandlePlayerEntryGameOk(cmd gnet.PacketCommand, msg *pb.PlayerEntryGameOk) { 
	// logic code ...
}
```

## 协程
每个玩家分配一个独立的逻辑协程,玩家在自己的逻辑协程中执行只涉及自身数据的代码,无需加锁

## 运行
安装mongodb

安装redis,单机模式和集群模式均可

修改config目录下的配置文件

编译运行

## 测试
测试客户端[gtestclient](https://github.com/fish-tennis/gtestclient)

## 编码规范参考
[https://github.com/uber-go/guide](https://github.com/uber-go/guide)

[https://github.com/xxjwxc/uber_go_guide_cn](https://github.com/xxjwxc/uber_go_guide_cn)

## 客户端网络库
C#: [gnet_csharp](https://github.com/fish-tennis/gnet_csharp)

## 讨论
QQ群: 764912827

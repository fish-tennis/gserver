# gserver
分布式游戏服务器框架

## 设计思路
- 类似gorm的数据绑定,使业务逻辑和数据库操作解耦
- 采用Entity-Component模式,尽可能使模块解耦
- 使用protobuf做通讯协议和数据序列化
- 玩家数据的存储接口可替换(mongodb,mysql,redis)
- 工具生成辅助代码,提供更友好的调用接口
- 网络库使用[github.com/fish-tennis/gnet](https://github.com/fish-tennis/gnet)

## 演示功能
- 服务器自动组网
- 服务器负载均衡
- 客户端直连模式和网关模式可选,网关模式支持WebSocket
- 一个账号同时只能登录一个服务器(数据一致性前提)
- 游戏服宕机后重启,自动修复缓存数据,防止玩家数据回档
- 通过反射自动注册消息回调,事件响应接口(业务代码和网络库解耦)
- 采用Entity-Component设计,模块解耦
- Entity事件分发
- 业务层和数据层分离,业务代码无需操作数据库和缓存
- 通过公会功能演示如何开发分布式的功能
- 通过公会功能演示服务器动态扩缩容的处理方式
- 服务器之间的rpc调用(类似grpc)
- 任务模块,演示了如何实现一个通用且扩展性强的[任务系统](https://github.com/fish-tennis/gserver/blob/main/Design_Quest.md)
- 活动模块,演示了如何设计一个通用且支持扩展的活动模块
- 配置数据管理模块,同时支持csv和json,支持热更新

## 数据方案
玩家数据落地使用mongodb,玩家上线时,从mongodb拉取玩家数据,玩家下线时,把玩家数据保存到mongodb

缓存使用redis,玩家在线期间修改的数据,即时保存到redis,防止服务器crash导致数据丢失

gserver提供了数据绑定的方案,业务层只需要标记哪些数据需要保存,无需自己写代码操作数据库和redis

## 数据绑定
类似gorm(go Object Relation Mapping)对SQL进行对象映射,gserver使用的数据绑定对组件进行数据库和缓存的映射

使用go的struct tag,设置对象组件的字段,框架接口会自动对这些字段进行数据库读取保存和缓存更新,极大的简化了业务代码对数据库和缓存的操作

设置组件保存数据
```go
// 玩家的一个组件
type Money struct {
  PlayerDataComponent
  // 该字段必须导出(首字母大写)
  // 使用struct tag来标记该字段需要存数据库
  Data *pb.Money `db:""`
}
```

支持明文方式保存数据
```go
// 玩家基础信息组件
type BaseInfo struct {
  PlayerDataComponent
  // plain表示明文存储,在保存到mongo时,不会进行proto序列化,以便于mongo语句直接操作
  Data *pb.BaseInfo `db:"plain"`
}
```

支持多个保存字段
```go
// 玩家的任务组件
type Quest struct {
  BasePlayerComponent
  // 保存数据的子模块:已完成的任务 使用明文保存方式
  // wrapper of []int32
  Finished *gentity.SliceData[int32] `child:"Finished;plain"`
  // 保存数据的子模块:当前任务列表
  // wrapper of map[int32]*pb.QuestData
  Quests *gentity.MapData[int32, *pb.QuestData] `child:"Quests"`
}
```

## 消息回调,事件响应
支持自动注册消息回调,事件响应
```go
// 客户端发给服务器的完成任务的消息回调
// 这种格式写的函数可以自动注册客户端消息回调
func (q *Quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
  // logic code ...
  return &pb.FinishQuestRes{ QuestCfgId: id, }, nil
}
```
```go
// 这种格式写的函数可以自动注册非客户端的消息回调
func (b *BaseInfo) HandlePlayerEntryGameOk(msg *pb.PlayerEntryGameOk) { 
  // logic code ...
}
```
```go
// 这种格式写的函数可以自动注册事件响应接口
// 当执行player.FireEvent(&EventPlayerEntryGame{})时,该响应接口会被调用
func (q *Quest) TriggerPlayerEntryGame(event *EventPlayerEntryGame) {
  // logic code ...
}
```

## rpc
```go
// 客户端请求查看自己所在公会的信息
func (g *Guild) OnGuildDataViewReq(req *pb.GuildDataViewReq) (*pb.GuildDataViewRes, error) {
  if 玩家还没加入公会 {
    return nil, errors.New("not a guild member")
  }
  // 向公会所在服务器发起rpc
  reply := new(pb.GuildDataViewRes)
  err := g.RouteRpcToSelfGuild(req, reply)
  return reply, err
}

// 公会服务响应rpc请求
func (g *GuildBaseInfo) HandleGuildDataViewReq(m *GuildMessage, req *pb.GuildDataViewReq) (*pb.GuildDataViewRes, error) {
  if 请求玩家不是本公会成员 {
    return nil, errors.New("not a member")
  }
  return &pb.GuildDataViewRes{...}, nil
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

欢迎有如下兴趣的小伙伴加入

- 客户端demo
- 服务器框架改进
- mysql的db接口实现
- 非redis的缓存cache接口实现
- 文档和示例
- 工具demo(如配置编辑等)
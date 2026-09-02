# gserver

分布式游戏服务器框架

## 设计思路

- 分布式服务器框架,无单点

- 类似gorm的数据绑定,使业务逻辑和数据库操作解耦

- 采用Entity-Component模式,尽可能使模块解耦

- 使用protobuf做通讯协议和数据序列化

- 玩家数据的存储接口可替换(mongodb,mysql,redis)

- 网络库使用[github.com/fish-tennis/gnet](https://github.com/fish-tennis/gnet)

## 演示功能

- 服务器自动组网,负载均衡

- 客户端直连模式和网关模式可选,支持WebSocket

- 一个账号同时只能登录一个服务器(数据一致性前提)

- 游戏服宕机后重启,自动修复缓存数据,防止玩家数据回档

- 自动注册消息回调,事件响应接口(业务代码和网络库解耦)

- 采用Entity-Component设计,模块解耦

- Entity事件分发

- 业务层和数据层分离,业务代码无需操作数据库和缓存

- 通过公会功能演示如何开发分布式的功能

- 通过公会功能演示服务器动态扩缩容的处理方式

- 服务器之间的rpc调用(类似grpc)

- 任务模块,演示了如何实现一个通用且扩展性强的[任务系统](https://github.com/fish-tennis/gserver/blob/main/Design_Quest.md)

- 容器模块,演示了常见的几种容器的实现(如道具背包,装备背包)

- 物品使用接口

- 兑换模块,如购买物品,兑换礼包,领取奖励等

- 活动模块,演示了如何设计一个通用且支持扩展的活动模块

- 配置数据管理模块,同时支持json和pb,支持热更新

- 网络协议的消息号自动生成

- 离线玩家数据的处理

- 玩家数据库的分片设计

- 全局类的非玩家实体的通用接口

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

## 玩家分片设计

玩家表(player)使用mongodb分片集群存储,分片键为`_id`(playerId,hashed):

- 按playerId的读写含分片键,mongos直达目标分片,天然无广播

- 玩家数据按playerId均匀散列到各分片,支撑玩家量水平扩展

分片带来的核心矛盾是**按账号查角色**(登录/进游/建角的高频路径):player表的AccountId查询不含分片键,会被mongos广播到所有分片;而"一个账号在一个区服只能建1个角色"的防重约束,也无法用player表的{AccountId,RegionId}复合唯一索引实现——mongodb要求分片集合的唯一索引必须以分片键为前缀。

gserver用一张独立的**账号区服映射表**(account\_player,不分片)同时解决这两个问题(设计说明见`db/account_player.go`):

- `_id`直接编码为`"{accountId}_{regionId}"`:相同\_id必然路由到同一个分片/节点,由数据库原子拒绝重复插入,唯一性全局保证,天然规避分片集合的唯一索引限制

- 建角流程:先insert映射(\_id冲突=该账号该区服已有角色,数据库层原子裁决),再insert player文档,player写入失败则回滚delete映射;映射是持久事实,建角成功后永久保留

- 按账号查角色:先查映射表(按\_id直达)拿到playerId,再按\_id操作player表,全程无广播

```go
// 进游:查映射表获取该账号在此区服的角色id(替代player表的按AccountId查询)
playerId, err := db.FindPlayerIdByAccount(accountId, req.GetRegionId())
```

- 登录的角色列表采用两段式:映射表按\_id前缀范围查(`["{accountId}_", "{accountId}_\xff")`)该账号所有区服的角色id,再按`_id $in`批量查player表的角色名/等级,每个查询各自直达

- player表另建{AccountId,RegionId}复合索引(**非唯一**,分片集群创建合法,唯一性由映射表承担),仅供GM后台按账号搜玩家等运维查询使用

分片键与主键不同的场景(如以AccountId为分片键),gentity v1.7.0提供了`ShardKeyEntityDb`/`ShardKeyProvider`可选接口,按id读写时自动附加分片键条件直达目标分片,详见[gentity](https://github.com/fish-tennis/gentity)的README

## 协程

每个玩家分配一个独立的逻辑协程,玩家在自己的逻辑协程中执行只涉及自身数据的代码,无需加锁

## 运行

安装mongodb

安装redis,单机模式和集群模式均可

修改config目录下的配置文件

编译运行

```shell
# 启动一个网关服
gserver -d=true -cfgDir=/cfgdata -conf=/gate_1.yaml
# 启动一个登录服
gserver -d=true -cfgDir=/cfgdata -conf=/login_11.yaml
#启动一个游戏服
gserver -d=true -cfgDir=/cfgdata -conf=/gamer_101.yaml
```

## 测试客户端

go控制台测试客户端[gtestclient](https://github.com/fish-tennis/gtestclient)

c#控制台测试客户端[cshap\_client](https://github.com/fish-tennis/cshap_client)

unity测试客户端[unity\_client](https://github.com/fish-tennis/unity_client)

## 编码规范参考

<https://github.com/uber-go/guide>

<https://github.com/xxjwxc/uber_go_guide_cn>

## 客户端网络库

C#: [gnet\_csharp](https://github.com/fish-tennis/gnet_csharp)

## Excel表导出工具

[excelexporter](https://github.com/fish-tennis/excelexporter)

gserver的excel/tool目录下有编译好的执行程序

## proto预处理工具

[proto\_code\_gen](https://github.com/fish-tennis/proto_code_gen)

在gserver项目中负责自动生成网络协议号

proto文件修改后,运行proto\tool\generate\_proto\_all.bat

generate\_proto\_all集成了protoc和proto\_code\_gen

## docker

快速构建1个测试环境:在docker/dev目录下运行(docker/dev/config下配置测试环境的参数)

docker-compose up -d

单独打包gserver

docker build -t gserver:latest .

## 讨论

QQ群: 764912827

欢迎有如下兴趣的小伙伴加入

- 客户端demo

- 服务器框架改进

- mysql的db接口实现

- 非redis的缓存cache接口实现

- 文档和示例

- 工具demo(如配置编辑等)


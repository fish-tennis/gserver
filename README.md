# gserver
使用[gnet](https://github.com/fish-tennis/gnet)开发的分布式游戏服务器框架

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

## 数据绑定

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

## 部署

安装mongodb

安装redis,单机模式和集群模式均可

修改config目录下的配置文件

编译运行

## 测试

## 编码规范参考
[https://github.com/uber-go/guide](https://github.com/uber-go/guide)

[https://github.com/xxjwxc/uber_go_guide_cn](https://github.com/xxjwxc/uber_go_guide_cn)

## TODO
- 全局对象的数据绑定

## 联系
QQ群: 764912827

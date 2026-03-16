---
name: "player-component-generator"
description: "生成Go语言玩家组件的初始代码。当用户想要创建带有客户端请求处理函数的新玩家组件时调用。"
---

# 玩家组件生成器

此技能用于生成遵循项目规范的玩家组件初始代码。

## 调用时机

- 用户想要创建新的玩家组件
- 用户要求为游戏生成新组件
- 用户需要玩家功能的样板代码

## 使用方法

向用户询问以下信息：
1. **组件名称**：组件的名称（例如："BaseInfo"、"Exchange"、"Quest"）
2. **客户端请求处理函数**：来自protobuf的Req/Res消息对列表
3. **数据存储类型**：
   - 无持久化数据,组件继承BasePlayerComponent
   - **Single Proto**：组件数据是单个protobuf消息（例如：`*pb.XxxData`）
     - 如果是明文存储,则数据字段使用`db:"plain"`标签,否则使用`db:""`标签
   - **MapData**：组件数据是使用`gentity.MapData`的map结构
     - 如果是MapData，还需要询问**Map键类型**：`int32`或`int64`或`string`

## 生成的代码结构

`该技能生成的Go文件包含：`

1. **包声明**：`package game`
2. **导入**：项目的标准导入
3. **常量**：组件名称常量
4. **init()函数**：组件注册
5. **结构体定义**：带有BasePlayerComponent或PlayerDataComponent的组件结构体
6. **Getter方法**：Player的`Get<ComponentName>()`方法
7. **SyncDataToClient()**：客户端同步方法
8. **请求处理函数**：每个客户端请求的`On<Xxx>Req()`方法

## 实现步骤

### 步骤1：检查Proto定义

检查`pb/`目录中是否存在所需的proto消息：
- 在`pb/*.pb.go`中搜索`<ComponentName>Data`
- 搜索客户端处理函数的Req/Res消息

### 步骤2：检查并更新PlayerData

检查`proto/player.proto`：
- 查找`message PlayerData`定义
- 检查组件字段是否存在（例如：`<ComponentName> <ComponentName>`或`bytes <ComponentName>`或`map<int32,bytes> <ComponentName>`或`map<int64,bytes> <ComponentName>`或`map<string,bytes> <ComponentName>`）
- 如果未找到，则将字段添加到PlayerData消息中
  - **Single Proto**：组件数据是单个protobuf消息（例如：`*pb.XxxData`）
    - 如果是明文存储,在字段为<ComponentName> <ComponentName>,否则为`bytes <ComponentName>`
  - **MapData**：组件数据是使用`gentity.MapData`的map结构
    - 如果是MapData，根据**Map键类型**,字段为`map<**Map键类型**,bytes> <ComponentName>`

### 步骤3：生成组件文件

使用适当的模板在`game/<componentname>.go`创建组件文件。

## 代码模板

### Single Proto Data（PlayerDataComponent）

```go
package game

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/pb"
)

const (
	ComponentName<ComponentName> = "<ComponentName>"
)

func init() {
	_playerComponentRegister.Register(ComponentName<ComponentName>, 0, func(player *Player, _ any) gentity.Component {
		return &<ComponentName>{
			PlayerDataComponent: *NewPlayerDataComponent(player, ComponentName<ComponentName>),
			Data: &pb.<ComponentName>Data{
				// TODO: 初始化默认值
			},
		}
	})
}

type <ComponentName> struct {
	PlayerDataComponent
	Data *pb.<ComponentName>Data `db:"plain"`
}

func (p *Player) Get<ComponentName>() *<ComponentName> {
	return p.GetComponentByName(ComponentName<ComponentName>).(*<ComponentName>)
}

func (c *<ComponentName>) SyncDataToClient() {
	c.GetPlayer().Send(&pb.<ComponentName>Sync{
		Data: c.Data,
	})
}

func (c *<ComponentName>) On<Xxx>Req(req *pb.<Xxx>Req) (*pb.<Xxx>Res, error) {
	l := c.GetPlayer().Log
	l.Debug("On<Xxx>Req", "req", req)
	// TODO: 实现逻辑
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

### MapData with int32 key（BasePlayerComponent）

```go
package game

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/pb"
)

const (
	ComponentName<ComponentName> = "<ComponentName>"
)

func init() {
	_playerComponentRegister.Register(ComponentName<ComponentName>, 0, func(player *Player, _ any) gentity.Component {
		return &<ComponentName>{
			BasePlayerComponent: BasePlayerComponent{
                player: player,
                name:   ComponentName<ComponentName>,
            },
			<DataField>: gentity.NewMapData[int32, *pb.<DataType>](),
		}
	})
}

type <ComponentName> struct {
	BasePlayerComponent
	<DataField> *gentity.MapData[int32, *pb.<DataType>] `db:""`
}

func (p *Player) Get<ComponentName>() *<ComponentName> {
	return p.GetComponentByName(ComponentName<ComponentName>).(*<ComponentName>)
}

func (c *<ComponentName>) SyncDataToClient() {
	c.GetPlayer().Send(&pb.<ComponentName>Sync{
		<DataField>: c.<DataField>.Data,
	})
}

func (c *<ComponentName>) On<Xxx>Req(req *pb.<Xxx>Req) (*pb.<Xxx>Res, error) {
	l := c.GetPlayer().Log
	l.Debug("On<Xxx>Req", "req", req)
	// TODO: 实现逻辑
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

### MapData with string key（BasePlayerComponent）

```go
package game

import (
	"errors"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/pb"
)

const (
	ComponentName<ComponentName> = "<ComponentName>"
)

func init() {
	_playerComponentRegister.Register(ComponentName<ComponentName>, 0, func(player *Player, _ any) gentity.Component {
		return &<ComponentName>{
			BasePlayerComponent: BasePlayerComponent{
                player: player,
                name:   ComponentName<ComponentName>,
            },
			<DataField>: gentity.NewMapData[string, *pb.<DataType>](),
		}
	})
}

type <ComponentName> struct {
	BasePlayerComponent
	<DataField> *gentity.MapData[string, *pb.<DataType>] `db:""`
}

func (p *Player) Get<ComponentName>() *<ComponentName> {
	return p.GetComponentByName(ComponentName<ComponentName>).(*<ComponentName>)
}

func (c *<ComponentName>) SyncDataToClient() {
	c.GetPlayer().Send(&pb.<ComponentName>Sync{
		<DataField>: c.<DataField>.Data,
	})
}

func (c *<ComponentName>) On<Xxx>Req(req *pb.<Xxx>Req) (*pb.<Xxx>Res, error) {
	l := c.GetPlayer().Log
	l.Debug("On<Xxx>Req", "req", req)
	// TODO: 实现逻辑
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

## 示例

对于带有ExchangeReq/ExchangeRes的"Exchange"组件：

1. 检查`pb/exchange.pb.go`中的消息定义
2. 检查`cfg/data_mgr.go`中的配置数据
3. 检查`proto/player.proto`中的PlayerData，如果缺少则添加`map<int32,bytes> Exchange`
4. 在`game/exchange.go`生成组件文件

## 注意事项

- **组件基类类型**：
  - 对于有持久化数据的组件使用`PlayerDataComponent`（Single Proto）
  - 对于有持久化数据的组件使用`BasePlayerComponent`（MapData）
  - 对于没有任何持久化数据的组件仅使用`BasePlayerComponent`
- **数据存储类型**：
  - **Single Proto**：带有`db:"plain"`标签的数据字段，用于明文存储（例如：`*pb.BaseInfo`）,带有`db:""`标签的数据字段，用于序列化存储
  - **MapData[int32, *pb.Xxx]**：用于以int32为键的数据（例如：任务id、物品id）
  - **MapData[int64, *pb.Xxx]**：用于以int64为键的数据（例如：唯一id）
  - **MapData[string, *pb.Xxx]**：用于以string为键的数据（例如：名称）
- **Proto文件**：
  - Proto定义位于`proto/`目录
  - `player.proto`包含带有所有组件字段的`PlayerData`消息
  - 修改proto文件后，运行`protoc`重新生成Go代码
- 遵循`game/base_info.go`、`game/exchange.go`、`game/bags.go`、`game/quest.go`中的现有模式
- 检查`pb/`目录中的protobuf定义以了解Req/Res消息结构

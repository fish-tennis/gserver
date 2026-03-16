---
name: "bag-generator"
description: "生成Go语言的背包/容器初始代码。当用户想要创建具有特定容器类型的新背包类型时调用。"
---

# 背包生成器

此技能用于生成遵循项目规范的背包/容器初始代码。

## 调用时机

- 用户想要创建新的背包/容器
- 用户要求生成新的物品栏类型
- 用户需要背包功能的样板代码

## 所需信息

向用户询问以下信息：
1. **背包名称**：背包的名称（例如："Equip"、"UniqueItem"）
2. **容器类型**：继承哪个ElemContainer：
   - **CountContainer**：用于可堆叠物品（键：int32，值：数量）
   - **UniqueContainer**：用于带有uniqueId的唯一物品（键：int64）
   - **CfgContainer**：用于基于配置的物品（键：int32，配置id）
3. **Proto数据类型**：如果使用UniqueContainer或CfgContainer，指定proto消息类型（例如：`*pb.Equip`、`*pb.HeroData`）

## 命名规范

根据背包名称生成：
- **结构体名称**：`<BagName>Bag`（例如："Equip" → "EquipBag"）
- **容器枚举**：`ContainerType_ContainerType_<BagName>`（例如："Equip" → `ContainerType_ContainerType_Equip`）
- **物品类型枚举**：`ItemType_ItemType_<BagName>`（例如："Equip" → `ItemType_ItemType_Equip`）
- **成员变量**：`Bag<BagName>`（例如："Equip" → `BagEquip`）

## 实现步骤

### 步骤1：检查并更新Proto文件

检查以下proto文件（位于`proto/`文件夹）：

1. **`proto/bags.proto`** - 检查`enum ContainerType`：
   - 如果容器枚举不存在，添加：`ContainerType_<BagName> = <next_value>`
   
2. **`proto/cfg.proto`** - 检查`enum ItemType`：
   - 如果物品类型枚举不存在，添加：`ItemType_<BagName> = <next_value>`
   
3. **`proto/bags.proto`** - 检查`message BagsSync`：
   - 添加带有适当map类型的背包字段

### 步骤2：创建背包文件

使用适当的模板创建`game/bag_<bagname>.go`。

### 步骤3：更新game/bags.go

1. 向`Bags`结构体添加背包字段：
   ```go
   Bag<BagName> *<BagName>Bag `child:"<BagName>"`
   ```

2. 在`init()`函数中初始化背包：
   ```go
   bags.Bag<BagName> = New<BagName>Bag(bags)
   ```

3. 在`GetBagByArg()`中添加case：
   ```go
   case int32(pb.ItemType_ItemType_<BagName>):
       return b.Bag<BagName>
   ```

4. 将背包添加到`SyncDataToClient()`：
   ```go
   <BagName>: b.Bag<BagName>.Data,
   ```

## 代码模板

### CountContainer模板

```go
package game

import "github.com/fish-tennis/gserver/pb"

type <BagName>Bag struct {
	*CountContainer `db:""`
}

func New<BagName>Bag(bags *Bags) *<BagName>Bag {
	bag := &<BagName>Bag{
		CountContainer: NewBagCountItem(bags),
	}
	return bag
}
```

### UniqueContainer模板

```go
package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gserver/pb"
)

type <BagName>Bag struct {
	*UniqueContainer[*pb.<DataType>] `db:""`
}

func New<BagName>Bag(bags *Bags) *<BagName>Bag {
	bag := &<BagName>Bag{
		UniqueContainer: NewBagUnique[*pb.<DataType>](bags, pb.ContainerType_ContainerType_<BagName>, func(arg *pb.AddElemArg) *pb.<DataType> {
			return &pb.<DataType>{
				Id: arg.GetId(),
                UniqueId: util.GenUniqueId(),
				// TODO: 初始化其他字段
			}
		}),
	}
	return bag
}
```

### CfgContainer模板

```go
package game

import "github.com/fish-tennis/gserver/pb"

type <BagName>Bag struct {
	*CfgContainer[*pb.<DataType>] `db:""`
}

func New<BagName>Bag(bags *Bags) *<BagName>Bag {
	bag := &<BagName>Bag{
		CfgContainer: NewBagCfg[*pb.<DataType>](bags, pb.ContainerType_ContainerType_<BagName>, func(arg *pb.AddElemArg) *pb.<DataType> {
			return &pb.<DataType>{
				Id: arg.GetId(),
				// TODO: 初始化其他字段
			}
		}),
	}
	return bag
}
```

## 示例

对于使用CfgContainer和`*pb.PetData`的"Pet"背包：

1. 检查`proto/bags.proto` - 添加`ContainerType_Pet = 6`
2. 检查`proto/cfg.proto` - 添加`ItemType_Pet = 4`
3. 检查`proto/bags.proto` - 向BagsSync添加`Pet map[int32]*PetData`
4. 创建`game/bag_pet.go`
5. 更新`game/bags.go`：
   - 添加`BagPet *PetBag \`child:"Pet"\``
   - 添加`bags.BagPet = NewPetBag(bags)`
   - 为`ItemType_ItemType_Pet`添加case
   - 向SyncDataToClient添加`Pet: b.BagPet.Data`

## 注意事项

- Proto文件位于`proto/`目录
- 修改proto文件后，运行`protoc`重新生成Go代码
- 遵循`game/bag_equip.go`中的现有模式
- ContainerType值必须唯一且连续
- ItemType值必须唯一且连续

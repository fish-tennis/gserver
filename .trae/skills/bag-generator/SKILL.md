---
name: "bag-generator"
description: "Generates initial code for bag/container in Go. Invoke when user wants to create a new bag type with specific container type."
---

# Bag Generator

This skill generates initial code for bag/container following the project's conventions.

## When to Invoke

- User wants to create a new bag/container
- User asks to generate a new inventory type
- User needs boilerplate code for a bag feature

## Required Information

Ask the user for:
1. **Bag Name**: The name of the bag (e.g., "Equip", "UniqueItem")
2. **Container Type**: Which ElemContainer to inherit from:
   - **CountContainer**: For stackable items (key: int32, value: count)
   - **UniqueContainer**: For unique items with uniqueId (key: int64)
   - **CfgContainer**: For config-based items (key: int32, config id)
3. **Proto Data Type**: If using UniqueContainer or CfgContainer, specify the proto message type (e.g., `*pb.Equip`, `*pb.UniqueCountItem`)

## Naming Conventions

Based on bag name, generate:
- **Struct Name**: `<BagName>Bag` (e.g., "Equip" → "EquipBag")
- **Container Enum**: `ContainerType_ContainerType_<BagName>` (e.g., "Equip" → `ContainerType_ContainerType_Equip`)
- **Item Type Enum**: `ItemType_ItemType_<BagName>` (e.g., "Equip" → `ItemType_ItemType_Equip`)
- **Member Variable**: `Bag<BagName>` (e.g., "Equip" → `BagEquip`)

## Implementation Steps

### Step 1: Check and Update Proto Files

Check the following proto files (located in parent directory's `proto/` folder):

1. **`proto/bags.proto`** - Check `enum ContainerType`:
   - If container enum doesn't exist, add: `ContainerType_<BagName> = <next_value>`
   
2. **`proto/cfg.proto`** - Check `enum ItemType`:
   - If item type enum doesn't exist, add: `ItemType_<BagName> = <next_value>`
   
3. **`proto/bags.proto`** - Check `message BagsSync`:
   - Add the bag field with appropriate map type

### Step 2: Create Bag File

Create `game/bag_<bagname>.go` with appropriate template.

### Step 3: Update game/bags.go

1. Add bag field to `Bags` struct:
   ```go
   Bag<BagName> *<BagName>Bag `child:"<BagName>"`
   ```

2. Initialize bag in `init()` function:
   ```go
   bags.Bag<BagName> = New<BagName>Bag(bags)
   ```

3. Add case in `GetBagByArg()`:
   ```go
   case int32(pb.ItemType_ItemType_<BagName>):
       return b.Bag<BagName>
   ```

4. Add bag to `SyncDataToClient()`:
   ```go
   <BagName>: b.Bag<BagName>.Data,
   ```

## Code Templates

### CountContainer Template

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

### UniqueContainer Template

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
				CfgId: arg.GetCfgId(),
                UniqueId: util.GenUniqueId(),
				// TODO: Initialize other fields
			}
		}),
	}
	return bag
}
```

### CfgContainer Template

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
				CfgId: arg.GetCfgId(),
				// TODO: Initialize other fields
			}
		}),
	}
	return bag
}
```

## Example

For a "Pet" bag using CfgContainer with `*pb.PetData`:

1. Check `proto/bags.proto` - Add `ContainerType_Pet = 6`
2. Check `proto/cfg.proto` - Add `ItemType_Pet = 4`
3. Check `proto/bags.proto` - Add `Pet map[int32]*PetData` to BagsSync
4. Create `game/bag_pet.go`
5. Update `game/bags.go`:
   - Add `BagPet *PetBag \`child:"Pet"\``
   - Add `bags.BagPet = NewPetBag(bags)`
   - Add case for `ItemType_ItemType_Pet`
   - Add `Pet: b.BagPet.Data` to SyncDataToClient

## Notes

- Proto files are in `../proto/` directory (parent of project root)
- After modifying proto files, run `protoc` to regenerate Go code
- Follow existing patterns in `game/bag_equip.go`
- ContainerType values must be unique and sequential
- ItemType values must be unique and sequential

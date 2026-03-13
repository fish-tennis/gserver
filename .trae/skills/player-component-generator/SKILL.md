---
name: "player-component-generator"
description: "Generates initial code for player components in Go. Invoke when user wants to create a new player component with client request handlers."
---

# Player Component Generator

This skill generates initial code for player components following the project's conventions.

## When to Invoke

- User wants to create a new player component
- User asks to generate a new component for the game
- User needs boilerplate code for a player feature

## Usage

Ask the user for:
1. **Component Name**: The name of the component (e.g., "BaseInfo", "Exchange", "Quest")
2. **Client Request Handlers**: List of Req/Res message pairs from protobuf
3. **Data Storage Type**: 
   - **Single Proto**: Component data is a single protobuf message (e.g., `*pb.XxxData`)
   - **MapData**: Component data is a map structure using `gentity.MapData`
     - If MapData, also ask for **Map Key Type**: `int32` or `string`

## Generated Code Structure

The skill generates a Go file with:

1. **Package declaration**: `package game`
2. **Imports**: Standard imports for the project
3. **Constants**: Component name constant
4. **init() function**: Component registration
5. **Struct definition**: Component struct with BasePlayerComponent or PlayerDataComponent
6. **Getter method**: `Get<ComponentName>()` for Player
7. **SyncDataToClient()**: Method for client synchronization
8. **Request handlers**: `On<Xxx>Req()` methods for each client request

## Implementation Steps

### Step 1: Check Proto Definitions

Check if the required proto messages exist in `pb/` directory:
- Search for `<ComponentName>Data` in `pb/*.pb.go`
- Search for Req/Res messages for client handlers

### Step 2: Check and Update PlayerData

Check `../proto/player.proto` (parent directory's proto folder):
- Look for `message PlayerData` definition
- Check if the component field exists (e.g., `<componentName> *<ComponentName>Data`)
- If not found, add the field to PlayerData message

### Step 3: Generate Component File

Create the component file at `game/<componentname>.go` using the appropriate template.

## Code Template

### Single Proto Data (PlayerDataComponent)

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
			PlayerDataComponent: PlayerDataComponent{
				BasePlayerComponent: BasePlayerComponent{
					player: player,
					name:   ComponentName<ComponentName>,
				},
			},
			Data: &pb.<ComponentName>Data{
				// TODO: Initialize default values
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
	// TODO: Implement logic
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

### MapData with int32 key (BasePlayerComponent)

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
	// TODO: Implement logic
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

### MapData with string key (BasePlayerComponent)

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
	// TODO: Implement logic
	res := &pb.<Xxx>Res{}
	return res, nil
}
```

## Example

For a "Exchange" component with ExchangeReq/ExchangeRes:

1. Check `pb/exchange.pb.go` for message definitions
2. Check `cfg/data_mgr.go` for configuration data
3. Check `../proto/player.proto` for PlayerData, add `exchange *ExchangeData` if missing
4. Generate the component file at `game/exchange.go`

## Notes

- **Component Base Types**:
  - Use `PlayerDataComponent` for components with persistent data (Single Proto)
  - Use `BasePlayerComponent` for components with persistent data (MapData)
  - Use `BasePlayerComponent` only for components without any persistent data
- **Data Storage Types**:
  - **Single Proto**: Data field with `db:"plain"` tag for plain storage (e.g., `*pb.BaseInfo`)
  - **MapData[int32, *pb.Xxx]**: For data keyed by int32 (e.g., quest id, item id)
  - **MapData[int64, *pb.Xxx]**: For data keyed by int64 (e.g., unique ids)
  - **MapData[string, *pb.Xxx]**: For data keyed by string (e.g., names)
- **Proto Files**:
  - Proto definitions are in `../proto/` directory (parent of project root)
  - `player.proto` contains `PlayerData` message with all component fields
  - After modifying proto files, run `protoc` to regenerate Go code
- Follow existing patterns in `game/base_info.go`, `game/exchange.go`, `game/bags.go`, `game/quest.go`
- Check protobuf definitions in `pb/` directory for Req/Res message structures

---
name: "client-handler"
description: "Generates client request/response handler methods in Go. Invoke when user wants to add a new client message handler to an existing component."
---

# Client Handler Generator

This skill generates client request handler methods for player components following the project's conventions.

## When to Invoke

- User wants to add a new client request handler
- User asks to add a Req/Res message handler
- User needs to implement client-server communication for a component

## Required Information

Ask the user for:
1. **Component Name**: The component to add the handler to (e.g., "Quest", "Guild", "Bags")
2. **Event Name**: The event name for the handler
   - Client request proto: `<EventName>Req`
   - Server response proto: `<EventName>Res`
   - Example: Event name "FinishQuest" → `FinishQuestReq` / `FinishQuestRes`

## Naming Conventions

- **Handler Method**: `On<EventName>Req`
- **Request Type**: `pb.<EventName>Req`
- **Response Type**: `pb.<EventName>Res`

Example: Event name "FinishQuest" → `OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error)`

## Implementation Steps

### Step 1: Verify Proto Definitions

Check if the Req/Res proto messages exist in `pb/` directory:
- Search for `<EventName>Req` and `<EventName>Res` in `pb/*.pb.go`
- If not found, inform user that proto definitions are missing

### Step 2: Locate Component File

Find the component file in `game/` directory:
- Component "Quest" → `game/quest.go`
- Component "Guild" → `game/guild.go`
- Component "Bags" → `game/bags.go`

### Step 3: Add Handler Method

Add the handler method to the component struct.

## Code Template

```go
func (c *<ComponentName>) On<EventName>Req(req *pb.<EventName>Req) (*pb.<EventName>Res, error) {
	l := c.GetPlayer().Log
	l.Debug("On<EventName>Req", "req", req)
	// TODO: Implement logic
	res := &pb.<EventName>Res{
		// TODO: Set response fields
	}
	return res, nil
}
```

## Example

For component "Quest" with event name "FinishQuest":

1. Check `pb/quest.pb.go` for `FinishQuestReq` and `FinishQuestRes`
2. Open `game/quest.go`
3. Add handler:

```go
func (q *quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
	l := q.GetPlayer().Log
	l.Debug("OnFinishQuestReq", "req", req)
	// TODO: Implement quest finish logic
	res := &pb.FinishQuestRes{
	}
	return res, nil
}
```

## Notes

- The handler method must be added to the correct component struct
- Method receiver should use a short variable name (e.g., `q` for Quest, `b` for Bags)
- Always include logging at the start of the handler
- Return appropriate error messages for validation failures
- Follow existing patterns in `game/quest.go`, `game/bags.go`
- Proto messages must be defined before adding the handler
- The framework automatically registers handlers based on method naming convention `On<Xxx>Req`

---
name: "client-handler"
description: "生成Go语言的客户端请求/响应处理方法。当用户想要向现有组件添加新的客户端消息处理函数时调用。"
---

# 客户端处理函数生成器

此技能用于生成遵循项目规范的玩家组件客户端请求处理方法。

## 调用时机

- 用户想要添加新的客户端请求处理函数
- 用户要求添加Req/Res消息处理函数
- 用户需要为组件实现客户端-服务器通信

## 所需信息

向用户询问以下信息：
1. **组件名称**：要添加处理函数的组件（例如："Quest"、"Guild"、"Bags"）
2. **事件名称**：处理函数的事件名称
   - 客户端请求proto：`<EventName>Req`
   - 服务器响应proto：`<EventName>Res`
   - 示例：事件名称"FinishQuest" → `FinishQuestReq` / `FinishQuestRes`

## 命名规范

- **处理方法**：`On<EventName>Req`
- **请求类型**：`pb.<EventName>Req`
- **响应类型**：`pb.<EventName>Res`

示例：事件名称"FinishQuest" → `OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error)`

## 实现步骤

### 步骤1：验证Proto定义

检查`pb/`目录中是否存在Req/Res proto消息：
- 在`pb/*.pb.go`中搜索`<EventName>Req`和`<EventName>Res`
- 如果未找到，通知用户proto定义缺失

### 步骤2：定位组件文件

在`game/`目录中查找组件文件：
- 组件"Quest" → `game/quest.go`
- 组件"Guild" → `game/guild.go`
- 组件"Bags" → `game/bags.go`

### 步骤3：添加处理方法

将处理方法添加到组件结构体中。

## 代码模板

```go
func (c *<ComponentName>) On<EventName>Req(req *pb.<EventName>Req) (*pb.<EventName>Res, error) {
	l := c.GetPlayer().Log
	l.Debug("On<EventName>Req", "req", req)
	// TODO: 实现逻辑
	res := &pb.<EventName>Res{
		// TODO: 设置响应字段
	}
	return res, nil
}
```

## 示例

对于组件"Quest"和事件名称"FinishQuest"：

1. 检查`pb/quest.pb.go`中的`FinishQuestReq`和`FinishQuestRes`
2. 打开`game/quest.go`
3. 添加处理函数：

```go
func (q *quest) OnFinishQuestReq(req *pb.FinishQuestReq) (*pb.FinishQuestRes, error) {
	l := q.GetPlayer().Log
	l.Debug("OnFinishQuestReq", "req", req)
	// TODO: 实现任务完成逻辑
	res := &pb.FinishQuestRes{
	}
	return res, nil
}
```

## 注意事项

- 处理方法必须添加到正确的组件结构体中
- 方法接收者应使用简短的变量名（例如：Quest用`q`，Bags用`b`）
- 始终在处理函数开始时包含日志记录
- 为验证失败返回适当的错误消息
- 遵循`game/quest.go`、`game/bags.go`中的现有模式
- Proto消息必须在添加处理函数之前定义
- 框架根据方法命名规范`On<Xxx>Req`自动注册处理函数

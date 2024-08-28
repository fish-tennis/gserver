package internal

const (
	// 客户端回调接口函数名前缀
	ClientHandlerMethodNamePrefix = "On"
	// 其他回调接口函数名前缀
	HandlerMethodNamePrefix = "Handle"
	// 事件响应接口函数名前缀
	EventHandlerMethodNamePrefix = "Trigger"
	// 事件分发嵌套层次限制
	SameEventLoopLimit = int32(3)
)

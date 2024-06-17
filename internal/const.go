package internal

const (
	// 对应proto文件里的package导出名
	ProtoPackageName = "gserver"
	// 客户端回调接口函数名前缀
	ClientHandlerMethodNamePrefix = "On"
	// 其他回调接口函数名前缀
	HandlerMethodNamePrefix = "Handle"
	// 事件响应接口函数名前缀
	EventHandlerMethodNamePrefix = "OnEvent"
	// 事件分发嵌套层次限制
	EventLoopLimit = int32(3)
)

package cache

import "context"

// 热更配置通知频道
const reloadConfigChannel = "notify:reload_config"

// PublishReloadConfig 发布热更配置通知
// 实现已通用化,发布逻辑见pubsub.go的PublishChannel
func PublishReloadConfig() error {
	return PublishChannel(context.Background(), reloadConfigChannel, "1")
}

// SubscribeReloadConfig 订阅热更配置通知
// 启动后台协程监听频道,收到消息时调用onReload;断线自动重试,ctx取消时退出
// onReload在订阅协程中执行,应尽快返回(重载配置等耗时操作可直接执行,后续通知会排队)
// 单条pub消息触发的panic由通用订阅逻辑(pubsub.go的SafeInvoke)捕获,不会导致服务器异常退出
// 实现已通用化,订阅/重连/panic防护逻辑见pubsub.go的SubscribeChannel
func SubscribeReloadConfig(ctx context.Context, onReload func()) {
	SubscribeChannel(ctx, reloadConfigChannel, func(_ string) { onReload() })
}

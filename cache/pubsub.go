package cache

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"github.com/redis/go-redis/v9"
)

// SafeInvoke panic安全地执行回调
// 回调通常运行在后台协程中:回调内部(如配置解析、外部钩子)的panic一旦外泄,
// 会杀死整个进程,影响面远超单次调用失败;
// 捕获后仅丢弃当次回调并记录日志,调用方流程与后续回调不受影响
// 注:此处不能调用internal.SendAlert上报(cache被internal反向依赖,会形成循环import)
func SafeInvoke(name string, fn func()) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("SafeInvoke: callback panic,已捕获,服务器继续运行", "name", name, "panic", r, "stack", string(debug.Stack()))
		}
	}()
	fn()
}

// PublishChannel 向指定频道发布消息
// 通用的发布入口,调用方只需关心频道名与payload,不必直接依赖GetRedis
func PublishChannel(ctx context.Context, channel, payload string) error {
	return GetRedis().Publish(ctx, channel, payload).Err()
}

// newChannelSubscription 创建频道订阅
// Subscribe是具体类型(*Client/*ClusterClient)的方法,不在Cmdable接口上,需类型断言
func newChannelSubscription(ctx context.Context, channel string) *redis.PubSub {
	switch client := GetRedis().(type) {
	case *redis.Client:
		return client.Subscribe(ctx, channel)
	case *redis.ClusterClient:
		return client.Subscribe(ctx, channel)
	default:
		slog.Error("newChannelSubscription: unsupported redis client type", "type", fmt.Sprintf("%T", GetRedis()))
		return nil
	}
}

// SubscribeChannel 订阅频道,收到消息时在订阅协程中调用onMessage(payload)
// 启动后台协程监听频道,断线自动重连,ctx取消时静默退出
//
// 双层panic防护(为什么需要两层):
//  1. 内层用SafeInvoke包裹onMessage:单条消息的回调panic只影响当次处理,
//     订阅循环继续收后续消息,不需要付出断线重连的代价
//  2. 外层给单轮订阅循环函数加recover:循环内非回调代码(如库内部状态异常)意外panic时,
//     若无此防护,协程会静默死亡且外部无任何感知,订阅能力永久丢失;
//     recover后结束当轮循环,走下方退避重连逻辑重启订阅
func SubscribeChannel(ctx context.Context, channel string, onMessage func(payload string)) {
	go func() {
		for {
			if ctx.Err() != nil {
				return
			}
			// 单轮订阅循环:正常收消息、出错或panic时返回,由下方逻辑决定是否重连
			subscribeLoopOnce(ctx, channel, onMessage)
			// 断线后稍等再重试,避免Redis不可用时疯狂重连
			// 也是订阅循环panic后的退避重启点,保证订阅能力不丢失
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
		}
	}()
}

// subscribeLoopOnce 执行单轮订阅循环:建立订阅并持续收消息,出错时返回由外层重连
// 单独拆成函数是为了让recover只覆盖一轮循环:panic发生在哪一轮,
// 就从哪一轮之后重启订阅,而不会把整个订阅协程带崩
func subscribeLoopOnce(ctx context.Context, channel string, onMessage func(payload string)) {
	defer func() {
		// 外层防护:循环内非回调代码意外panic,记录后结束本轮,由外层退避重启订阅
		if r := recover(); r != nil {
			slog.Error("SubscribeChannel: subscribe loop panic,已捕获,稍后重启订阅", "channel", channel, "panic", r, "stack", string(debug.Stack()))
		}
	}()

	sub := newChannelSubscription(ctx, channel)
	if sub == nil {
		// 不支持的Redis客户端类型,直接返回由外层统一退避3秒后重试
		// (进程可能先Init本模块、后配置Redis,首次断言失败不代表永远失败)
		return
	}
	defer sub.Close()

	for {
		msg, err := sub.ReceiveMessage(ctx)
		if err != nil {
			// ctx取消导致的退出,静默返回;其他错误记录日志后交由外层重新订阅
			if ctx.Err() != nil {
				return
			}
			slog.Error("SubscribeChannel receive error, retrying", "channel", channel, "error", err)
			return
		}
		slog.Info("SubscribeChannel: received message", "channel", channel)
		// 内层防护:单条消息回调panic只被SafeInvoke捕获,订阅循环继续收后续消息
		SafeInvoke("SubscribeChannel:"+channel, func() { onMessage(msg.Payload) })
	}
}

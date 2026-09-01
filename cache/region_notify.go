package cache

import (
	"context"
	"log/slog"
	"strconv"
)

// region_notify.go 区服数据更新通知
//
// 背景:区服数据以 MongoDB 为唯一权威数据源,各进程(GameServer/LoginServer)持有内存快照;
// GM 后台修改区服后调用本文件的通知机制广播,各进程收到通知后重读 MongoDB 刷新本地快照。
//
// 通知语义为 at-most-once(至多一次):
// Redis Pub/Sub 不持久化消息,订阅方断线/重连窗口内发布的通知会永久丢失,
// 且订阅方无法感知自己错过了什么。这是本机制固有、且被明确接受的特性。
//
// 补偿手段:
// GM 发布侧与各进程订阅接收侧的日志均携带 regionId,可离线比对两侧日志发现缺失的通知;
// 发现缺失后,可在 GM 后台重新执行一次区服编辑(幂等操作)触发新通知,完成补偿。
//
// 为什么不做轮询兜底:
// 区服变更是低频运维操作,订阅断线窗口恰好撞上变更的概率很低,
// 经权衡接受该小概率风险,以避免为极低频场景引入额外的轮询机制及其复杂度。
//
// 数据安全性:
// 即使通知丢失,MongoDB 中的权威数据不受任何影响,
// 仅进程内快照暂时陈旧,进程重启后从 MongoDB 重新加载会自动纠正。

// 区服数据更新通知频道
// 命名风格与 reload_notify.go 的 reloadConfigChannel 保持一致(notify: 前缀 + snake_case)
const regionUpdateChannel = "notify:region_update"

// PublishRegionUpdate 发布区服数据更新通知
// 由 GM 后台在修改区服并落库 MongoDB 成功后调用,payload 为 regionId 的十进制字符串
// 发布成功与否的日志由调用方负责记录,本函数不记日志
// 实现已通用化,发布逻辑见 pubsub.go 的 PublishChannel
func PublishRegionUpdate(ctx context.Context, regionId int32) error {
	return PublishChannel(ctx, regionUpdateChannel, strconv.FormatInt(int64(regionId), 10))
}

// SubscribeRegionUpdate 订阅区服数据更新通知
// 启动后台协程监听频道,收到消息后解析出 regionId 并调用 onRegionId,由其重读 MongoDB 刷新快照
// 断线自动重试,ctx取消时退出;onRegionId 在订阅协程中执行,应尽快返回
// payload 解析失败时记录 Warn 日志并跳过该消息,不回调(不中断订阅,后续消息正常处理)
// 单条消息回调 panic 由通用订阅逻辑(pubsub.go 的 SafeInvoke)捕获,不会导致服务器异常退出
// 实现已通用化,订阅/重连/panic防护逻辑见 pubsub.go 的 SubscribeChannel
func SubscribeRegionUpdate(ctx context.Context, onRegionId func(regionId int32)) {
	SubscribeChannel(ctx, regionUpdateChannel, func(payload string) {
		regionId, err := strconv.ParseInt(payload, 10, 32)
		if err != nil {
			// 收到非预期格式的payload:仅丢弃当次消息,订阅关系保持不变
			slog.Warn("SubscribeRegionUpdate: invalid regionId payload, skip message", "payload", payload, "error", err)
			return
		}
		onRegionId(int32(regionId))
	})
}

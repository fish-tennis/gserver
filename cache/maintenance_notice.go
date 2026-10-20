package cache

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/fish-tennis/gserver/pb"
)

// maintenanceNoticeChannel 维护预告的Pub/Sub频道
// 发布方:GM后台/运维脚本,如:
//
//	redis-cli PUBLISH maintenance_notice '{"ShutdownTimestamp":1730000000,"DurationSec":300,"Version":"1.2.0"}'
//
// 订阅方:所有game进程,收到后广播给本进程在线玩家(见GameServer.BroadcastMaintenanceNotice)
const maintenanceNoticeChannel = "maintenance_notice"

// maintenanceNoticePayload 频道载荷(json格式,与pb.ServerMaintenanceNotice字段对应)
type maintenanceNoticePayload struct {
	ShutdownTimestamp int64  `json:"ShutdownTimestamp"`
	DurationSec       int32  `json:"DurationSec"`
	Version           string `json:"Version"`
}

// PublishMaintenanceNotice 发布维护预告(所有game进程会收到并广播给在线玩家)
func PublishMaintenanceNotice(ctx context.Context, notice *pb.ServerMaintenanceNotice) error {
	payload, err := json.Marshal(&maintenanceNoticePayload{
		ShutdownTimestamp: notice.GetShutdownTimestamp(),
		DurationSec:       notice.GetDurationSec(),
		Version:           notice.GetVersion(),
	})
	if err != nil {
		return err
	}
	return PublishChannel(ctx, maintenanceNoticeChannel, string(payload))
}

// SubscribeMaintenanceNotice 订阅维护预告,收到后在订阅协程中调用onNotice
// 断线自动重连(见SubscribeChannel),ctx取消时退出
func SubscribeMaintenanceNotice(ctx context.Context, onNotice func(notice *pb.ServerMaintenanceNotice)) {
	SubscribeChannel(ctx, maintenanceNoticeChannel, func(payload string) {
		var data maintenanceNoticePayload
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			slog.Error("SubscribeMaintenanceNotice: invalid payload", "payload", payload, "err", err)
			return
		}
		onNotice(&pb.ServerMaintenanceNotice{
			ShutdownTimestamp: data.ShutdownTimestamp,
			DurationSec:       data.DurationSec,
			Version:           data.Version,
		})
	})
}

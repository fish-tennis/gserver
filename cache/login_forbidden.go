package cache

import (
	"context"
	"encoding/json"
	"log/slog"
)

// loginForbiddenChannel 禁止登录指令的Pub/Sub频道
// 发布方:GM后台/运维脚本,如:
//
//	redis-cli PUBLISH forbidden_login '{"Forbidden":true}'              # 全量置位(滚动更新)
//	redis-cli PUBLISH forbidden_login '{"Forbidden":true,"ServerIds":[105]}'  # 仅置位105(降配)
//
// 订阅方:所有game进程,收到后SetLocalLoginForbidden并随心跳上报(见GameServer.Init)
//
// 用途一(滚动更新):发布前全量置位,配合"进程重启后LoginForbidden自动恢复false"的特性,
// 使重启完成的game自动成为唯一可登录的新版候选,玩家/新登录只会流向已更新的服
// 用途二(降配缩容):仅对目标服置位——切忌全量置位,否则目标服玩家fallback无候选、
// 原服又永久下线,形成无法恢复的死锁
// 更新中途放弃时手动发布{"Forbidden":false}解除(同样支持ServerIds定向解除)
const loginForbiddenChannel = "forbidden_login"

// loginForbiddenPayload 频道载荷(json)
// ServerIds为空时对所有game生效;非空时仅对这些ServerId的game生效
type loginForbiddenPayload struct {
	Forbidden bool    `json:"Forbidden"`
	ServerIds []int32 `json:"ServerIds"`
}

// PublishLoginForbidden 发布禁止登录指令(所有game进程会收到,按ServerIds过滤后更新心跳上报的标记)
func PublishLoginForbidden(ctx context.Context, forbidden bool, serverIds ...int32) error {
	payload, err := json.Marshal(&loginForbiddenPayload{Forbidden: forbidden, ServerIds: serverIds})
	if err != nil {
		return err
	}
	return PublishChannel(ctx, loginForbiddenChannel, string(payload))
}

// SubscribeLoginForbidden 订阅禁止登录指令,收到后在订阅协程中调用onForbidden
// localServerId为本进程的ServerId,用于ServerIds定向指令的过滤
// 断线自动重连(见SubscribeChannel),ctx取消时退出
func SubscribeLoginForbidden(ctx context.Context, localServerId int32, onForbidden func(forbidden bool)) {
	SubscribeChannel(ctx, loginForbiddenChannel, func(payload string) {
		var data loginForbiddenPayload
		if err := json.Unmarshal([]byte(payload), &data); err != nil {
			slog.Error("SubscribeLoginForbidden: invalid payload", "payload", payload, "err", err)
			return
		}
		if len(data.ServerIds) > 0 {
			// 定向指令:仅对列表内的ServerId生效
			matched := false
			for _, serverId := range data.ServerIds {
				if serverId == localServerId {
					matched = true
					break
				}
			}
			if !matched {
				slog.Debug("SubscribeLoginForbidden: not in ServerIds, ignore", "localServerId", localServerId)
				return
			}
		}
		onForbidden(data.Forbidden)
	})
}

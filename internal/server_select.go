package internal

import (
	"math/rand"

	"github.com/fish-tennis/gserver/pb"
)

// WeightedSelectGameServer 按剩余容量(MaxOnline-OnlineCount)加权随机选一个游戏服
// loginserver(账号登录分配)与gate(无缝重启fallback)共用,保证选服口径一致
//
// 为什么用加权随机而不是别的策略:
//   - 纯随机:样本小(开服洪峰/单服宕机迁移)时分布不均,个别服被压垮而其他服空闲
//   - 最小在线优先:心跳数据有秒级延迟,洪峰下并发请求会全部命中快照里"最轻"的同一台(惊群)
//   - 加权随机:剩余容量越大被选中概率越高,兼顾均衡;随机性天然把并发请求分散开
//
// 满载(剩余容量<=0)与禁止登录(LoginForbidden=true,正在退出/灰度屏蔽)的服务器不参与抽签;
// 全部不可用返回nil,调用方按各自语义处理(loginserver返回TryLater;gate放弃fallback)——
// 宁可让玩家等,也不把已满的服压垮、或把玩家送上正在退出的服(开服/停服场景的保护闸门)
func WeightedSelectGameServer(servers []*pb.ServerInfo) *pb.ServerInfo {
	totalRemaining := int64(0)
	for _, serverInfo := range servers {
		if serverInfo.GetLoginForbidden() {
			continue
		}
		if remaining := ServerRemainingCapacity(serverInfo); remaining > 0 {
			totalRemaining += remaining
		}
	}
	if totalRemaining == 0 {
		// 空列表、全部满载或全部禁止登录
		return nil
	}
	lottery := rand.Int63n(totalRemaining)
	for _, serverInfo := range servers {
		if serverInfo.GetLoginForbidden() {
			continue
		}
		remaining := ServerRemainingCapacity(serverInfo)
		if remaining <= 0 {
			continue
		}
		if lottery < remaining {
			return serverInfo
		}
		lottery -= remaining
	}
	// 理论上不可达:totalRemaining>0时循环内必然命中,防御性返回避免潜在panic路径
	return nil
}

// ServerRemainingCapacity 计算服务器剩余可分配容量,负数归零
// MaxOnline<=0视为未配置,按默认容量计算(保证未配置的存量Game服不会被误判为满载)
func ServerRemainingCapacity(serverInfo *pb.ServerInfo) int64 {
	maxOnline := serverInfo.GetMaxOnline()
	if maxOnline <= 0 {
		maxOnline = DefaultGameServerMaxOnline
	}
	remaining := int64(maxOnline) - int64(serverInfo.GetOnlineCount())
	if remaining < 0 {
		return 0
	}
	return remaining
}

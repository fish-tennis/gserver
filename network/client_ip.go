package network

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/util"
)

// ResolveClientIp 统一解析客户端真实 IP
//
// 网关模式(packet 是 GatePacket):后端处理器拿到的 connection 是网关与后端之间的连接,
// 其 RemoteAddr 是网关 IP 而非客户端真实 IP,因此客户端 IP 必须由网关在转发时写入请求体的
// ClientIp 字段,这里直接返回调用方从请求体取出的 reqClientIp。
//
// 直连模式(非 GatePacket):connection 即客户端连接,直接从其 RemoteAddr 提取客户端 IP。
func ResolveClientIp(connection Connection, packet Packet, reqClientIp string) string {
	if IsGatePacket(packet) {
		return reqClientIp
	}
	return util.GetIpFromAddr(connection.RemoteAddr())
}

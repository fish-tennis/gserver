package util

import "net"

// GetLocalIP 获取本机出口 IP 地址
// 使用 UDP 连接法:向任意外部地址发起 UDP 连接,读取本地端地址即为出口 IP
// 这种方法不需要真正发送数据包,仅利用操作系统的路由表确定出口网卡
func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// GetIpFromAddr 从 net.Addr 中提取纯 IP 地址(去掉端口部分)
// 网关转发登录类请求时,用于从客户端连接的 RemoteAddr 提取客户端真实 IP
// nil 地址安全返回空串,SplitHostPort 失败(如地址不含端口)时回退返回原始字符串
func GetIpFromAddr(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		// 地址不含端口或格式异常时,直接返回原始字符串
		return addr.String()
	}
	return host
}

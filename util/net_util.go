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

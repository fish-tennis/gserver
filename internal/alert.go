package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"runtime/debug"
	"strings"
	"sync"
	"time"
)

// 全局告警管理器实例
var alertMgr = &AlertManager{}

// AlertInfo 告警携带的服务器元信息
type AlertInfo struct {
	ServerId   string `json:"ServerId"`
	ServerType string `json:"ServerType"`
	LocalIP    string `json:"LocalIP"`
	WorkDir    string `json:"WorkDir"`
	BuildTime  string `json:"BuildTime"`
	BuildType  string `json:"BuildType"`
	GitVersion string `json:"GitVersion"`
}

// AlertManager 告警管理器,负责向 webhook 发送 panic 告警信息
type AlertManager struct {
	enabled bool
	webhook string
	info    *AlertInfo
	mu      sync.Mutex
	dedup   map[string]bool // 排重: source file:line -> 是否已发送
}

// InitAlert 初始化告警模块
// webhook 为空时,告警功能不启用,所有 SendAlert 调用将静默跳过
func InitAlert(webhook string, serverId int32, serverType, localIP, workDir, buildTime, buildType, gitVersion string) {
	if webhook == "" {
		slog.Info("AlertModule disabled: no AlertWebhook configured")
		return
	}
	alertMgr.enabled = true
	alertMgr.webhook = webhook
	alertMgr.info = &AlertInfo{
		ServerId:   fmt.Sprintf("%d", serverId),
		ServerType: serverType,
		LocalIP:    localIP,
		WorkDir:    workDir,
		BuildTime:  buildTime,
		BuildType:  buildType,
		GitVersion: gitVersion,
	}
	alertMgr.dedup = make(map[string]bool)
	slog.Info("AlertModule initialized", "webhook", webhook)
}

// SendAlert 发送 panic 告警信息到 webhook
// err: recover() 捕获到的 panic 值
// 该函数异步发送,不会阻塞调用方
// 基于 panic 发生源(source file:line)排重,同一位置的 panic 只发送一次
func SendAlert(err interface{}) {
	if !alertMgr.enabled {
		return
	}
	stack := debug.Stack()

	// 从堆栈中解析排重 key (panic 发生源的 file:line)
	dedupKey := parseDedupKey(stack)

	alertMgr.mu.Lock()
	if alertMgr.dedup[dedupKey] {
		alertMgr.mu.Unlock()
		return
	}
	alertMgr.dedup[dedupKey] = true
	alertMgr.mu.Unlock()

	// 异步发送,避免阻塞业务协程
	go alertMgr.send(err, stack, dedupKey)
}

// 用于匹配堆栈中的 file:line
var fileLineRe = regexp.MustCompile(`\t(.+\.go):(\d+)`)

// parseDedupKey 从堆栈跟踪中解析排重 key
// 跳过标准库路径和告警模块自身的帧,取第一个业务代码帧作为排重 key
// 堆栈中第一个非 runtime/非 alert.go 的帧通常是 recover 处理函数,
// 第二个才是 panic 发生源;但为简化逻辑,这里取第一个业务帧
func parseDedupKey(stack []byte) string {
	matches := fileLineRe.FindAllSubmatch(stack, -1)
	for _, m := range matches {
		path := string(m[1])
		line := string(m[2])
		// 跳过标准库路径
		if strings.Contains(path, "/runtime/") {
			continue
		}
		// 跳过告警模块自身
		if strings.HasSuffix(path, "internal/alert.go") {
			continue
		}
		return path + ":" + line
	}
	return "unknown"
}

// send 实际发送 HTTP POST 请求到 webhook
func (a *AlertManager) send(err interface{}, stack []byte, dedupKey string) {
	// 构建告警消息文本
	text := fmt.Sprintf(
		"[服务器告警]\n"+
			"ServerType: %s\n"+
			"ServerId: %s\n"+
			"IP: %s\n"+
			"WorkDir: %s\n"+
			"BuildTime: %s\n"+
			"BuildType: %s\n"+
			"GitVersion: %s\n"+
			"Error: %v\n"+
			"Source: %s\n"+
			"Stack:\n%s",
		a.info.ServerType, a.info.ServerId, a.info.LocalIP,
		a.info.WorkDir, a.info.BuildTime, a.info.BuildType,
		a.info.GitVersion, err, dedupKey, string(stack),
	)

	// 采用钉钉/飞书等通用 webhook 格式
	body := map[string]interface{}{
		"msgtype": "text",
		"text": map[string]string{
			"content": text,
		},
	}
	jsonData, e := json.Marshal(body)
	if e != nil {
		slog.Error("AlertModule marshal error", "error", e)
		return
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, e := client.Post(a.webhook, "application/json", bytes.NewReader(jsonData))
	if e != nil {
		slog.Error("AlertModule send error", "error", e)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		slog.Error("AlertModule webhook non-200", "status", resp.StatusCode)
	}
}

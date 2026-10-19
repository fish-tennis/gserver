package metrics

import (
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 等待运维HTTP服务就绪(Start为异步启动)
func waitServerReady(t *testing.T, addr string) {
	t.Helper()
	for i := 0; i < 100; i++ {
		resp, err := http.Get("http://" + addr + "/metrics")
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("metrics server not ready")
}

func TestMetricsHttpServer(t *testing.T) {
	addr := "127.0.0.1:19399"
	Init("Test", 999)
	SetOnlineCount(3)
	IncPacketReceived()
	// 端口被占用时同步返回错误(调用方可即时告警,不再静默降级)
	blocker, err := net.Listen("tcp", "127.0.0.1:19398")
	if err != nil {
		t.Fatal(err)
	}
	defer blocker.Close()
	if err := Start("127.0.0.1:19398"); err == nil {
		t.Fatal("expected error when port is in use")
	}
	if err := Start(addr); err != nil {
		t.Fatal(err)
	}
	defer Shutdown()
	waitServerReady(t, addr)

	// /metrics 返回自定义指标
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	for _, name := range []string{
		"online_players",
		"packets_total",
		"goroutines",
		"db_worker_queue_len",
	} {
		if !strings.Contains(text, name) {
			t.Fatalf("metrics response missing %v", name)
		}
	}
	// 标签和值
	if !strings.Contains(text, `server_type="Test"`) || !strings.Contains(text, `server_id="999"`) {
		t.Fatal("metrics response missing server labels")
	}

	// pprof 首页
	pprofResp, err := http.Get("http://" + addr + "/debug/pprof/")
	if err != nil {
		t.Fatal(err)
	}
	defer pprofResp.Body.Close()
	if pprofResp.StatusCode != http.StatusOK {
		t.Fatalf("pprof index status: %v", pprofResp.StatusCode)
	}
	// pprof profile 端点(cpu profile 0秒采样)
	profileResp, err := http.Get("http://" + addr + "/debug/pprof/profile?seconds=1")
	if err != nil {
		t.Fatal(err)
	}
	defer profileResp.Body.Close()
	if profileResp.StatusCode != http.StatusOK {
		t.Fatalf("pprof profile status: %v", profileResp.StatusCode)
	}

	// Shutdown 后可重复 Start(模拟进程重启场景下包内状态复位)
	Shutdown()
	if httpServer != nil {
		t.Fatal("httpServer should be nil after Shutdown")
	}
}

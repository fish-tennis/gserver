// Package metrics 提供运维监控能力:
//   - Prometheus 指标暴露(/metrics): 在线人数/协程数/消息处理数/DB协程池积压等,
//     并自动附带 go_* / process_* 运行时指标(client_golang默认注册)
//   - pprof 在线性能分析(/debug/pprof/*)
//
// 两个能力共用同一个HTTP服务,由配置文件的 MetricsAddr 决定是否开启,
// 建议只监听内网地址(如 127.0.0.1:19001),避免暴露到公网
package metrics

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// 本进程的服务器标签(GaugeVec的标签值),Init时赋值
	serverLabels prometheus.Labels

	onlinePlayers = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "online_players",
		Help: "当前在线人数(Game:在线玩家数; Gate:已绑定的客户端连接数; 其他类型:0)",
	}, []string{"server_type", "server_id"})

	goroutines = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "goroutines",
		Help: "当前进程的协程数量(含每个玩家的独立逻辑协程)",
	})

	packetsReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "packets_total",
		Help: "处理的消息累计数(含客户端消息和服务器间消息),用rate()计算每秒消息数",
	})

	dbWorkerQueueLen = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "db_worker_queue_len",
		Help: "DB操作协程池的排队任务数,接近上限(8个worker*64队列容量)说明DB层积压",
	})

	// 运维HTTP服务(nil=未启动)
	httpServer *http.Server
)

func init() {
	prometheus.MustRegister(onlinePlayers, goroutines, packetsReceived, dbWorkerQueueLen)
}

// Init 设置进程的服务器标签,在BaseServer.Init中调用(先于Start)
func Init(serverType string, serverId int32) {
	serverLabels = prometheus.Labels{
		"server_type": serverType,
		"server_id":   strconv.Itoa(int(serverId)),
	}
}

// Start 启动运维HTTP服务:/metrics + /debug/pprof/*
func Start(addr string) {
	if httpServer != nil {
		return
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	// 手动注册pprof handler到独立的mux,避免污染http.DefaultServeMux
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	httpServer = &http.Server{Addr: addr, Handler: mux}
	go func() {
		slog.Info("metrics server start", "addr", addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("metrics server error", "addr", addr, "err", err)
			httpServer = nil
		}
	}()
}

// Shutdown 优雅关闭运维HTTP服务
func Shutdown() {
	if httpServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		slog.Warn("metrics server shutdown err", "err", err)
	}
	httpServer = nil
}

// SetOnlineCount 更新在线人数指标(每秒随心跳更新)
func SetOnlineCount(n int32) {
	if serverLabels == nil {
		return
	}
	onlinePlayers.With(serverLabels).Set(float64(n))
}

// SetGoroutines 更新协程数指标(每秒随心跳更新)
func SetGoroutines(n int) {
	goroutines.Set(float64(n))
}

// SetDbWorkerQueueLen 更新DB协程池排队数指标(每秒随心跳更新)
func SetDbWorkerQueueLen(n int) {
	dbWorkerQueueLen.Set(float64(n))
}

// IncPacketReceived 消息处理计数+1(热路径,单次原子加)
func IncPacketReceived() {
	packetsReceived.Inc()
}

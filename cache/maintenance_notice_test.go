package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/fish-tennis/gserver/pb"
)

// 维护预告的发布/订阅回路(miniredis支持Pub/Sub)
func TestMaintenanceNoticePublishSubscribe(t *testing.T) {
	server := miniredis.RunT(t)
	NewRedis([]string{server.Addr()}, "", "", false, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	noticeCh := make(chan *pb.ServerMaintenanceNotice, 1)
	SubscribeMaintenanceNotice(ctx, func(notice *pb.ServerMaintenanceNotice) {
		noticeCh <- notice
	})

	// 等订阅协程就绪再发布(订阅建立前发布的消息不会被收到)
	time.Sleep(100 * time.Millisecond)

	want := &pb.ServerMaintenanceNotice{
		ShutdownTimestamp: time.Now().Add(10 * time.Minute).Unix(),
		DurationSec:       300,
		Version:           "1.2.0",
	}
	if err := PublishMaintenanceNotice(ctx, want); err != nil {
		t.Fatal(err)
	}

	select {
	case got := <-noticeCh:
		if got.GetShutdownTimestamp() != want.GetShutdownTimestamp() ||
			got.GetDurationSec() != want.GetDurationSec() ||
			got.GetVersion() != want.GetVersion() {
			t.Fatalf("notice mismatch: got %v want %v", got, want)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscribe callback not triggered")
	}
}

package cache

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

// 禁止登录指令的发布/订阅回路(miniredis支持Pub/Sub)
func TestLoginForbiddenPublishSubscribe(t *testing.T) {
	server := miniredis.RunT(t)
	NewRedis([]string{server.Addr()}, "", "", false, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	forbiddenCh := make(chan bool, 2)
	SubscribeLoginForbidden(ctx, 101, func(forbidden bool) {
		forbiddenCh <- forbidden
	})

	// 等订阅协程就绪再发布
	time.Sleep(100 * time.Millisecond)

	// 置位
	if err := PublishLoginForbidden(ctx, true); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-forbiddenCh:
		if !got {
			t.Fatal("expected forbidden=true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscribe callback not triggered for true")
	}

	// 解除
	if err := PublishLoginForbidden(ctx, false); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-forbiddenCh:
		if got {
			t.Fatal("expected forbidden=false")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscribe callback not triggered for false")
	}
}

// 定向指令(ServerIds)只对列表内的ServerId生效
func TestLoginForbiddenServerIdsFilter(t *testing.T) {
	server := miniredis.RunT(t)
	NewRedis([]string{server.Addr()}, "", "", false, 0)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	targetCh := make(chan bool, 1) // ServerId=105:在ServerIds内,应收到
	otherCh := make(chan bool, 1)  // ServerId=101:不在ServerIds内,不应收到
	SubscribeLoginForbidden(ctx, 105, func(forbidden bool) { targetCh <- forbidden })
	SubscribeLoginForbidden(ctx, 101, func(forbidden bool) { otherCh <- forbidden })
	time.Sleep(100 * time.Millisecond)

	// 定向置位105(降配场景)
	if err := PublishLoginForbidden(ctx, true, 105); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-targetCh:
		if !got {
			t.Fatal("target server should receive forbidden=true")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("target server callback not triggered")
	}
	// 非目标服不应收到
	select {
	case got := <-otherCh:
		t.Fatalf("non-target server should not receive, got %v", got)
	case <-time.After(300 * time.Millisecond):
		// 预期:无消息
	}
}

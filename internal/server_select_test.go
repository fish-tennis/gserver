package internal

import (
	"testing"

	"github.com/fish-tennis/gserver/pb"
)

// 全部不可用(空列表/禁止登录/满载/超载)时返回nil,调用方按各自语义处理
func TestWeightedSelectGameServerNoneAvailable(t *testing.T) {
	if got := WeightedSelectGameServer(nil); got != nil {
		t.Fatalf("nil servers: expect nil, got %v", got.GetServerId())
	}
	forbidden := []*pb.ServerInfo{{ServerId: 1, LoginForbidden: true, MaxOnline: 100}}
	if got := WeightedSelectGameServer(forbidden); got != nil {
		t.Fatalf("all forbidden: expect nil, got %v", got.GetServerId())
	}
	full := []*pb.ServerInfo{{ServerId: 2, MaxOnline: 100, OnlineCount: 100}}
	if got := WeightedSelectGameServer(full); got != nil {
		t.Fatalf("all full: expect nil, got %v", got.GetServerId())
	}
	over := []*pb.ServerInfo{{ServerId: 3, MaxOnline: 100, OnlineCount: 150}}
	if got := WeightedSelectGameServer(over); got != nil {
		t.Fatalf("over capacity: expect nil, got %v", got.GetServerId())
	}
}

// 禁止登录/满载的服务器永远不会被选中
func TestWeightedSelectGameServerFilters(t *testing.T) {
	servers := []*pb.ServerInfo{
		{ServerId: 101, MaxOnline: 100, OnlineCount: 0},
		{ServerId: 102, LoginForbidden: true, MaxOnline: 100, OnlineCount: 0},
		{ServerId: 103, MaxOnline: 100, OnlineCount: 100},
	}
	for i := 0; i < 1000; i++ {
		got := WeightedSelectGameServer(servers)
		if got == nil {
			t.Fatal("only 101 is available, expect non-nil")
		}
		if got.GetServerId() != 101 {
			t.Fatalf("unavailable server %v selected", got.GetServerId())
		}
	}
}

// MaxOnline<=0视为未配置,按默认容量计算,不会被误判为满载
func TestWeightedSelectGameServerDefaultCapacity(t *testing.T) {
	available := []*pb.ServerInfo{{ServerId: 101, MaxOnline: 0, OnlineCount: DefaultGameServerMaxOnline - 1}}
	if got := WeightedSelectGameServer(available); got == nil || got.GetServerId() != 101 {
		t.Fatalf("unconfigured MaxOnline with default-capacity online should be selectable, got %v", got)
	}
	full := []*pb.ServerInfo{{ServerId: 102, MaxOnline: 0, OnlineCount: DefaultGameServerMaxOnline}}
	if got := WeightedSelectGameServer(full); got != nil {
		t.Fatalf("default capacity full: expect nil, got %v", got.GetServerId())
	}
}

// 选中概率近似剩余容量占比(101剩余100,102剩余1000,期望101被选中~9.1%)
func TestWeightedSelectGameServerWeightedDistribution(t *testing.T) {
	servers := []*pb.ServerInfo{
		{ServerId: 101, MaxOnline: 1000, OnlineCount: 900},
		{ServerId: 102, MaxOnline: 1000, OnlineCount: 0},
	}
	const total = 20000
	count101 := 0
	for i := 0; i < total; i++ {
		got := WeightedSelectGameServer(servers)
		if got == nil {
			t.Fatal("expect always selectable")
		}
		if got.GetServerId() == 101 {
			count101++
		}
	}
	ratio := float64(count101) / total
	// 大样本下二项分布标准差约0.2%,容忍区间7%~13%足够稳定
	if ratio < 0.07 || ratio > 0.13 {
		t.Fatalf("weighted ratio drift: server 101(remaining 100/1100) selected %.1f%%, expect ~9.1%%", ratio*100)
	}
}

package cache

import (
	"sync"
	"testing"
)

// ==================== 玩家上下线缓存接口的生命周期测试 ====================
// 覆盖在线玩家/在线账号全部公开接口的串联行为:
// AddOnlineAccount(独占) / AddOnlinePlayer(占有) / TakeOverOnlinePlayer(接管)
// RemoveOnlineAccount / RemoveOnlinePlayer(条件释放)
// GetOnlineAccount / GetOnlinePlayer / GetOnlinePlayers(查询)
// ResetOnlinePlayer(崩溃残留修复)
// 调用序列与业务层保持一致:
//   - 进游: entry_game_handler(processPlayerEntryGameReq) -> AddOnlineAccount -> game_server.AddPlayer -> AddOnlinePlayer
//   - 下线: game_server.RemovePlayer -> RemoveOnlineAccount -> RemoveOnlinePlayer
//   - 崩溃修复: game_server.repairCache -> ResetOnlinePlayer

// TestAddOnlineAccountExclusivity 账号独占语义(SetNX):
// 首次成功/重复失败/他服失败/释放后重新占有/条件释放不误删
func TestAddOnlineAccountExclusivity(t *testing.T) {
	setupOnlineTestRedis(t)
	// 首次占有成功
	if !AddOnlineAccount(100, 1000, 101) {
		t.Fatal("first add should succeed")
	}
	// 同服重复占有失败(独占)
	if AddOnlineAccount(100, 1000, 101) {
		t.Fatal("duplicate add on same server should fail")
	}
	// 他服占有失败
	if AddOnlineAccount(100, 1000, 102) {
		t.Fatal("add on other server should fail")
	}
	// 读取:值与写入一致
	if pid, gsid := GetOnlineAccount(100); pid != 1000 || gsid != 101 {
		t.Fatalf("GetOnlineAccount: %v %v", pid, gsid)
	}
	// 条件释放:旧参数(不匹配当前值)不生效
	RemoveOnlineAccount(100, 999, 101)
	RemoveOnlineAccount(100, 1000, 999)
	if pid, _ := GetOnlineAccount(100); pid != 1000 {
		t.Fatal("mismatch release should not del")
	}
	// 正确释放后可重新占有
	RemoveOnlineAccount(100, 1000, 101)
	if pid, _ := GetOnlineAccount(100); pid != 0 {
		t.Fatal("match release should del")
	}
	if !AddOnlineAccount(100, 1000, 102) {
		t.Fatal("re-add after release should succeed")
	}
}

// TestGetOnlinePlayersBatch 批量查询:在线/离线混合、空入参
func TestGetOnlinePlayersBatch(t *testing.T) {
	setupOnlineTestRedis(t)
	AddOnlinePlayer(1, 11, 101)
	AddOnlinePlayer(2, 22, 102)
	// 玩家3不在线
	m := GetOnlinePlayers([]int64{1, 2, 3})
	if len(m) != 2 {
		t.Fatalf("expected 2 online, got %v", m)
	}
	if m[1] != 101 || m[2] != 102 {
		t.Fatalf("server mapping wrong: %v", m)
	}
	if _, ok := m[3]; ok {
		t.Fatal("offline player should be absent")
	}
	// 空入参返回nil
	if GetOnlinePlayers(nil) != nil {
		t.Fatal("nil input should return nil")
	}
	if GetOnlinePlayers([]int64{}) != nil {
		t.Fatal("empty input should return nil")
	}
	// 下线后从批量结果消失
	RemoveOnlinePlayer(1, 101)
	m = GetOnlinePlayers([]int64{1, 2})
	if len(m) != 1 || m[2] != 102 {
		t.Fatalf("after offline: %v", m)
	}
}

// TestPlayerOnlineLifecycle 完整上下线生命周期(与业务层调用序列一致)
func TestPlayerOnlineLifecycle(t *testing.T) {
	setupOnlineTestRedis(t)
	const (
		accountId = int64(100)
		playerId  = int64(1000)
		gsid      = int32(101)
	)
	// ---- 进游:entry handler -> AddOnlineAccount;AddPlayer -> AddOnlinePlayer ----
	if !AddOnlineAccount(accountId, playerId, gsid) {
		t.Fatal("entry: AddOnlineAccount failed")
	}
	if !AddOnlinePlayer(playerId, accountId, gsid) {
		t.Fatal("entry: AddOnlinePlayer failed")
	}
	// ---- 在线:三种查询口径一致 ----
	if a, g := GetOnlinePlayer(playerId); a != accountId || g != gsid {
		t.Fatalf("GetOnlinePlayer: %v %v", a, g)
	}
	if p, g := GetOnlineAccount(accountId); p != playerId || g != gsid {
		t.Fatalf("GetOnlineAccount: %v %v", p, g)
	}
	if g, ok := GetOnlinePlayers([]int64{playerId})[playerId]; !ok || g != gsid {
		t.Fatalf("GetOnlinePlayers: %v %v", g, ok)
	}
	// ---- 重连:账号独占挡住重复进游 ----
	if AddOnlineAccount(accountId, playerId, gsid) {
		t.Fatal("reconnect: duplicate AddOnlineAccount should fail")
	}
	// 同服玩家记录可重入(幂等)
	if !AddOnlinePlayer(playerId, accountId, gsid) {
		t.Fatal("reconnect: same-server AddOnlinePlayer should succeed")
	}
	// ---- 下线:RemovePlayer -> RemoveOnlineAccount -> RemoveOnlinePlayer ----
	RemoveOnlineAccount(accountId, playerId, gsid)
	RemoveOnlinePlayer(playerId, gsid)
	if a, _ := GetOnlinePlayer(playerId); a != 0 {
		t.Fatalf("after logout player record: %v", a)
	}
	if p, _ := GetOnlineAccount(accountId); p != 0 {
		t.Fatalf("after logout account record: %v", p)
	}
	// ---- 下线后可立即重新上线 ----
	if !AddOnlineAccount(accountId, playerId, gsid) {
		t.Fatal("re-login AddOnlineAccount failed")
	}
	if !AddOnlinePlayer(playerId, accountId, gsid) {
		t.Fatal("re-login AddOnlinePlayer failed")
	}
}

// TestPlayerCrashResidueRepair 服务器崩溃残留的端到端修复(repairCache流程)
func TestPlayerCrashResidueRepair(t *testing.T) {
	setupOnlineTestRedis(t)
	// 模拟服务器101异常宕机:两个玩家的在线记录和集合索引残留
	for _, pid := range []int64{1000, 1001} {
		aid := pid - 900
		if !AddOnlineAccount(aid, pid, 101) {
			t.Fatalf("setup account %v", pid)
		}
		if !AddOnlinePlayer(pid, aid, 101) {
			t.Fatalf("setup player %v", pid)
		}
	}
	// 服务器101重启:repairCache -> ResetOnlinePlayer修复全部残留
	repaired := make(map[int64]int64)
	ResetOnlinePlayer(101, func(pid, aid int64) error {
		repaired[pid] = aid
		return nil
	})
	if len(repaired) != 2 || repaired[1000] != 100 || repaired[1001] != 101 {
		t.Fatalf("repaired: %v", repaired)
	}
	// 残留全部清干净
	for _, pid := range []int64{1000, 1001} {
		if a, _ := GetOnlinePlayer(pid); a != 0 {
			t.Fatalf("player residue not cleaned: %v -> %v", pid, a)
		}
	}
	for _, aid := range []int64{100, 101} {
		if p, _ := GetOnlineAccount(aid); p != 0 {
			t.Fatalf("account residue not cleaned: %v -> %v", aid, p)
		}
	}
	// 修复后玩家可正常上线
	if !AddOnlineAccount(100, 1000, 101) || !AddOnlinePlayer(1000, 100, 101) {
		t.Fatal("login after repair failed")
	}
}

// TestConcurrentOnlineRace 多服务器并发上下线竞态
// 每个协程模拟一台服务器的完整进游->下线序列,验证最终一致性:
// 结束后账号与玩家记录要么都清空,要么一致地在线(归属同一服)
// (条件释放保证交错时的下线清理不会误删其他服务器的新记录)
func TestConcurrentOnlineRace(t *testing.T) {
	setupOnlineTestRedis(t)
	const (
		accountId = int64(700)
		playerId  = int64(7000)
	)
	const goroutines = 8
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			gsid := int32(300 + idx)
			// 账号独占:同一时刻只有一个服务器能进游
			if !AddOnlineAccount(accountId, playerId, gsid) {
				return
			}
			// AddPlayer:占有失败则强制接管(与game_server.AddPlayer行为一致)
			if !AddOnlinePlayer(playerId, accountId, gsid) {
				TakeOverOnlinePlayer(playerId, accountId, gsid)
			}
			// 立即下线(RemovePlayer序列)
			RemoveOnlineAccount(accountId, playerId, gsid)
			RemoveOnlinePlayer(playerId, gsid)
		}(i)
	}
	wg.Wait()
	// 最终一致性断言
	a, g := GetOnlinePlayer(playerId)
	p, pg := GetOnlineAccount(accountId)
	if (a == 0) != (p == 0) {
		t.Fatalf("inconsistent state: player=%v/%v account=%v/%v", a, g, p, pg)
	}
	if a != 0 {
		if a != accountId || p != playerId {
			t.Fatalf("corrupted values: player=%v account=%v", a, p)
		}
		if g != pg {
			t.Fatalf("server mismatch: player=%v account=%v", g, pg)
		}
	}
}

// TestRealRedis_PlayerLifecycle 真实Redis(DB:1)上的完整生命周期(FCALL路径)
func TestRealRedis_PlayerLifecycle(t *testing.T) {
	newRealOnlineTestRedis(t)
	if !gserverFuncRunner.Available() {
		t.Skip("redis functions unsupported, skip FCALL lifecycle test")
	}
	const (
		accountId = int64(800)
		playerId  = int64(8000)
	)
	// 进游
	if !AddOnlineAccount(accountId, playerId, 701) {
		t.Fatal("AddOnlineAccount failed")
	}
	if !AddOnlinePlayer(playerId, accountId, 701) {
		t.Fatal("AddOnlinePlayer failed")
	}
	if g := GetOnlinePlayers([]int64{playerId})[playerId]; g != 701 {
		t.Fatalf("GetOnlinePlayers on FCALL: %v", g)
	}
	// 异服进游被独占挡住
	if AddOnlineAccount(accountId, playerId, 702) {
		t.Fatal("cross-server AddOnlineAccount should fail")
	}
	if AddOnlinePlayer(playerId, accountId, 702) {
		t.Fatal("cross-server AddOnlinePlayer should fail")
	}
	// 旧服崩溃:login server检测目标服宕机后,用旧值条件清理账号残留
	// (与loginserver/login_handler.go宕机清理路径一致)
	RemoveOnlineAccount(accountId, playerId, 701)
	// 新服进游:获得账号独占,玩家记录是旧服残留 -> claim失败 -> AddPlayer强制接管
	if !AddOnlineAccount(accountId, playerId, 702) {
		t.Fatal("new server AddOnlineAccount failed")
	}
	if AddOnlinePlayer(playerId, accountId, 702) {
		t.Fatal("claim old-server residue should fail")
	}
	oldAid, oldGsid := TakeOverOnlinePlayer(playerId, accountId, 702)
	if oldAid != accountId || oldGsid != 701 {
		t.Fatalf("takeover old value: %v %v", oldAid, oldGsid)
	}
	// 下线
	RemoveOnlineAccount(accountId, playerId, 702)
	RemoveOnlinePlayer(playerId, 702)
	if a, _ := GetOnlinePlayer(playerId); a != 0 {
		t.Fatalf("final state player: %v", a)
	}
	if p, _ := GetOnlineAccount(accountId); p != 0 {
		t.Fatalf("final state account: %v", p)
	}
	// FCALL路径断言
	if !gserverFuncRunner.Available() {
		t.Fatal("expected FCALL path")
	}
}

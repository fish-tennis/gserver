package cache

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// 使用miniredis验证在线玩家/在线账号的Lua脚本行为
// 生产环境为Redis Cluster,所有脚本均只操作单个key,天然满足集群的slot约束
func setupOnlineTestRedis(t *testing.T) {
	t.Helper()
	server := miniredis.RunT(t)
	NewRedis([]string{server.Addr()}, "", "", false, 0)
}

func TestClaimOnlinePlayer(t *testing.T) {
	setupOnlineTestRedis(t)
	// 无记录:占有成功
	if !AddOnlinePlayer(100, 1, 101) {
		t.Fatal("claim empty failed")
	}
	accountId, gsid := GetOnlinePlayer(100)
	if accountId != 1 || gsid != 101 {
		t.Fatalf("GetOnlinePlayer: %v %v", accountId, gsid)
	}
	// 本服记录(崩溃残留):可重新占有
	if !AddOnlinePlayer(100, 1, 101) {
		t.Fatal("re-claim own failed")
	}
	// 其他服务器持有:占有失败
	if AddOnlinePlayer(100, 1, 102) {
		t.Fatal("claim other's record should fail")
	}
	// 占有失败不回滚SAdd:调用方会接管记录为本服,集合索引保留才能保证宕机自修复
	members, err := GetRedis().SMembers(context.Background(), keyGameServerPlayer(102)).Result()
	if err != nil || len(members) != 1 {
		t.Fatalf("claim fail should keep SAdd for takeover: %v %v", members, err)
	}
	// 接管后:记录指向新服,且ResetOnlinePlayer(新服)可从集合索引找到并修复该玩家
	TakeOverOnlinePlayer(100, 1, 102)
	if accountId, gsid := GetOnlinePlayer(100); accountId != 1 || gsid != 102 {
		t.Fatalf("after takeover: %v %v", accountId, gsid)
	}
	ResetOnlinePlayer(102, func(playerId, accountId int64) error { return nil })
	if accountId, _ := GetOnlinePlayer(100); accountId != 0 {
		t.Fatalf("residue after takeover should be repairable via set index: %v", accountId)
	}
}

func TestReleaseOnlinePlayerConditional(t *testing.T) {
	setupOnlineTestRedis(t)
	if !AddOnlinePlayer(100, 1, 101) {
		t.Fatal("claim failed")
	}
	// 其他服务器的释放请求:不能删除本服记录
	RemoveOnlinePlayer(100, 102)
	if accountId, gsid := GetOnlinePlayer(100); accountId != 1 || gsid != 101 {
		t.Fatalf("other's release should not del: %v %v", accountId, gsid)
	}
	// 本服释放:删除成功
	RemoveOnlinePlayer(100, 101)
	if accountId, gsid := GetOnlinePlayer(100); accountId != 0 || gsid != 0 {
		t.Fatalf("own release should del: %v %v", accountId, gsid)
	}
}

func TestTakeOverOnlinePlayer(t *testing.T) {
	setupOnlineTestRedis(t)
	if !AddOnlinePlayer(100, 1, 101) {
		t.Fatal("claim failed")
	}
	// 原子接管:返回旧值并写入新值
	oldAccountId, oldGameServerId := TakeOverOnlinePlayer(100, 1, 102)
	if oldAccountId != 1 || oldGameServerId != 101 {
		t.Fatalf("takeover old value: %v %v", oldAccountId, oldGameServerId)
	}
	if accountId, gsid := GetOnlinePlayer(100); accountId != 1 || gsid != 102 {
		t.Fatalf("takeover new value: %v %v", accountId, gsid)
	}
	// 接管后,旧服务器的条件释放不会误删新记录
	RemoveOnlinePlayer(100, 101)
	if accountId, gsid := GetOnlinePlayer(100); accountId != 1 || gsid != 102 {
		t.Fatalf("old server's late release should not del new record: %v %v", accountId, gsid)
	}
}

func TestRemoveOnlineAccountConditional(t *testing.T) {
	setupOnlineTestRedis(t)
	if !AddOnlineAccount(1, 100, 101) {
		t.Fatal("AddOnlineAccount failed")
	}
	// 值不匹配(其他服务器的记录):不删除
	RemoveOnlineAccount(1, 100, 102)
	if playerId, gsid := GetOnlineAccount(1); playerId != 100 || gsid != 101 {
		t.Fatalf("mismatch release should not del: %v %v", playerId, gsid)
	}
	// 值匹配:删除成功
	RemoveOnlineAccount(1, 100, 101)
	if playerId, _ := GetOnlineAccount(1); playerId != 0 {
		t.Fatalf("match release should del: %v", playerId)
	}
}

func TestResetOnlinePlayerSkipsNonLocal(t *testing.T) {
	setupOnlineTestRedis(t)
	// 本服残留:玩家记录属于本服(101)
	if !AddOnlinePlayer(100, 1, 101) {
		t.Fatal("claim failed")
	}
	// 其他服务器(102)在线的玩家,但在本服(101)集合中留有残留元素
	GetRedis().SAdd(context.Background(), keyGameServerPlayer(101), 200)
	GetRedis().Set(context.Background(), keyOnlinePlayer(200), "2;102", 0)
	repaired := make(map[int64]int64)
	ResetOnlinePlayer(101, func(playerId, accountId int64) error {
		repaired[playerId] = accountId
		return nil
	})
	// 只修复属于本服的玩家100
	if len(repaired) != 1 || repaired[100] != 1 {
		t.Fatalf("repaired: %v", repaired)
	}
	// 玩家200的记录未被误删(旧实现会误删,把在线玩家"踢下线")
	if accountId, gsid := GetOnlinePlayer(200); accountId != 2 || gsid != 102 {
		t.Fatalf("non-local record must be kept: %v %v", accountId, gsid)
	}
	// 本服残留已清理
	if accountId, _ := GetOnlinePlayer(100); accountId != 0 {
		t.Fatalf("local residue should be cleaned: %v", accountId)
	}
	if playerId, _ := GetOnlineAccount(1); playerId != 0 {
		t.Fatalf("local account residue should be cleaned: %v", playerId)
	}
}

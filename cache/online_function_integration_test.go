package cache

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

// ==================== 真实Redis集成测试 ====================
// 依赖本地Redis实例(默认127.0.0.1:6379),连接失败时Skip
// NewRedis会自动安装gentity+gserver两个函数库;此处验证FCALL路径与库丢失后的EVAL回退

const testRealRedisAddr = "127.0.0.1:6379"

func newRealOnlineTestRedis(t *testing.T) {
	t.Helper()
	// 使用DB:1与默认DB隔离(本地单机redis测试约定)
	client := redis.NewClient(&redis.Options{Addr: testRealRedisAddr, DB: 1})
	if err := client.Ping(context.Background()).Err(); err != nil {
		client.Close()
		t.Skipf("local redis %v unavailable, skip integration test: %v", testRealRedisAddr, err)
	}
	client.Close()
	t.Cleanup(func() {
		// 模式删除测试残留(避免硬编码key列表遗漏),并删除函数库(下次NewRedis会重新安装)
		c := GetRedis()
		for _, pattern := range []string{"p.*", "onlineaccount:*", "onlineplayer:*", "game:*", "gtest:*"} {
			if keys, err := c.Keys(context.Background(), pattern).Result(); err == nil && len(keys) > 0 {
				c.Del(context.Background(), keys...)
			}
		}
		if client, ok := c.(*redis.Client); ok {
			client.FunctionDelete(context.Background(), "gserver")
			client.FunctionDelete(context.Background(), "gentity")
		}
	})
	// NewRedis内部自动执行函数库安装(失败回退EVAL);DB:1与默认DB隔离
	NewRedis([]string{testRealRedisAddr}, "", "", false, 1)
}

// TestGserverFunctionsFCall 安装成功后在线玩家/账号操作走FCALL且行为正确
func TestGserverFunctionsFCall(t *testing.T) {
	newRealOnlineTestRedis(t)
	if !gserverFuncRunner.Available() {
		t.Skipf("redis functions unsupported (redis<7.0?), skip FCALL test")
	}

	// 占有:无记录成功;重复占有成功;他服失败并回滚SAdd
	if !AddOnlinePlayer(9001, 8001, 701) {
		t.Fatal("claim empty failed")
	}
	if !AddOnlinePlayer(9001, 8001, 701) {
		t.Fatal("re-claim own failed")
	}
	if AddOnlinePlayer(9001, 8001, 702) {
		t.Fatal("claim other's should fail")
	}
	if accountId, gsid := GetOnlinePlayer(9001); accountId != 8001 || gsid != 701 {
		t.Fatalf("record corrupted: %v %v", accountId, gsid)
	}
	// 条件释放:他服请求不生效,本服释放生效
	RemoveOnlinePlayer(9001, 702)
	if accountId, _ := GetOnlinePlayer(9001); accountId != 8001 {
		t.Fatal("other's release should not del")
	}
	RemoveOnlinePlayer(9001, 701)
	if accountId, _ := GetOnlinePlayer(9001); accountId != 0 {
		t.Fatal("own release should del")
	}
	// 接管:返回旧值并写入新值
	AddOnlinePlayer(9002, 8001, 701)
	oldAid, oldGsid := TakeOverOnlinePlayer(9002, 8001, 702)
	if oldAid != 8001 || oldGsid != 701 {
		t.Fatalf("takeover old value: %v %v", oldAid, oldGsid)
	}
	// 账号条件释放:值不匹配不删,匹配才删
	AddOnlineAccount(8001, 9002, 702)
	RemoveOnlineAccount(8001, 9002, 701)
	if pid, _ := GetOnlineAccount(8001); pid != 9002 {
		t.Fatal("mismatch release should not del")
	}
	RemoveOnlineAccount(8001, 9002, 702)
	if pid, _ := GetOnlineAccount(8001); pid != 0 {
		t.Fatal("match release should del")
	}
}

// TestGserverFunctionsFallback 库被删除后自动回退EVAL,行为不变
// 回归场景:运维执行FUNCTION FLUSH/FUNCTION DELETE或故障切换导致库丢失
func TestGserverFunctionsFallback(t *testing.T) {
	newRealOnlineTestRedis(t)
	if !gserverFuncRunner.Available() {
		t.Skipf("redis functions unsupported (redis<7.0?), skip FCALL test")
	}
	// 制造库丢失
	client, ok := GetRedis().(*redis.Client)
	if !ok {
		t.Fatalf("unexpected client type %T", GetRedis())
	}
	if err := client.FunctionDelete(context.Background(), "gserver").Err(); err != nil {
		t.Fatalf("FunctionDelete: %v", err)
	}
	// 后续操作自动回退EVAL(而非返回Function not found错误),行为与FCALL一致
	if !AddOnlinePlayer(9001, 8001, 701) {
		t.Fatal("fallback claim failed")
	}
	if AddOnlinePlayer(9001, 8001, 702) {
		t.Fatal("fallback claim other's should fail")
	}
	if accountId, gsid := GetOnlinePlayer(9001); accountId != 8001 || gsid != 701 {
		t.Fatalf("fallback record corrupted: %v %v", accountId, gsid)
	}
	RemoveOnlinePlayer(9001, 701)
	if accountId, _ := GetOnlinePlayer(9001); accountId != 0 {
		t.Fatal("fallback release should del")
	}
	if gserverFuncRunner.Available() {
		t.Fatal("expected EVAL fallback state after library deleted")
	}
	// 重新调用initGserverFunctions可恢复FCALL路径
	initGserverFunctions()
	if !gserverFuncRunner.Available() {
		t.Fatal("expected FCALL restored after reinstall")
	}
}

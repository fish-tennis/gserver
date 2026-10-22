package cache

import (
	"testing"
	"time"

	"github.com/fish-tennis/gserver/pb"
)

// 使用miniredis验证登录session的续期语义(在线玩家周期续期,离线自然过期)
func TestRenewLoginSession(t *testing.T) {
	setupOnlineTestRedis(t)
	account := &pb.Account{Id: 100, Name: "test"}

	// key不存在时续期是no-op:EXPIRE不创建新key,不会为已过期账号凭空造出长效session
	RenewLoginSession(account.GetId())
	if ok, _ := VerifyLoginSession(account.GetId(), "whatever"); ok {
		t.Fatal("renew should not create session key")
	}

	// 登录生成session后,续期应把TTL重置回LoginSessionExpireTime
	session := NewLoginSession(account)
	if session == "" {
		t.Fatal("NewLoginSession failed")
	}
	// 模拟TTL已消耗一部分(离到期只剩1分钟),续期后应回到满TTL
	GetRedis().Expire(t.Context(), keyLoginSession(account.GetId()), time.Minute)
	RenewLoginSession(account.GetId())
	ttl, err := GetRedis().TTL(t.Context(), keyLoginSession(account.GetId())).Result()
	if err != nil || ttl < LoginSessionExpireTime-time.Second {
		t.Fatalf("renew should reset ttl to %v, got %v err=%v", LoginSessionExpireTime, ttl, err)
	}
	// 续期后session仍可验证通过(值未被改动)
	if ok, name := VerifyLoginSession(account.GetId(), session); !ok || name != account.GetName() {
		t.Fatalf("session should still verify after renew, ok=%v name=%v", ok, name)
	}
}

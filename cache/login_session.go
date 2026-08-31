package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"strconv"

	"github.com/fish-tennis/gserver/pb"
)

const (
	// time.Minute * 10 10分钟过期,根据实际情况调整
	LoginSessionExpireTime = time.Minute * 10
)

func keyLoginSession(accountId int64) string {
	return "ses:" + strconv.FormatInt(accountId, 10)
}

// NewLoginSession 新生成一个登录session
// 使用 crypto/rand 生成不可预测的随机 token,防止攻击者通过时间戳猜测 session 伪造登录
func NewLoginSession(account *pb.Account) string {
	session := generateRandomSession()
	// 登录session存redis,供玩家登录游戏服时验证用,使登录服和游戏服可以解耦
	_, err := GetRedis().SetEx(context.Background(), keyLoginSession(account.GetId()), session, LoginSessionExpireTime).Result()
	if IsRedisError(err) {
		slog.Error("NewLoginSession error", "error", err)
		return ""
	}
	return session
}

// generateRandomSession 生成32字符的十六进制随机token
// crypto/rand 失败时回退到纳秒时间戳,保证功能可用
func generateRandomSession() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		slog.Error("generateRandomSession rand failed", "err", err)
		// crypto/rand 失败极为罕见,回退到时间戳保证功能可用
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return hex.EncodeToString(buf)
}

// 验证登录session
func VerifyLoginSession(accountId int64, session string) bool {
	if session == "" {
		return false
	}
	cacheSession, err := GetRedis().Get(context.Background(), keyLoginSession(accountId)).Result()
	if IsRedisError(err) {
		return false
	}
	return cacheSession == session
}

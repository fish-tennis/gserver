package cache

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"strings"
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
//
// Redis存储值格式: {session}:{accountName}
// 把accountName一并存入session缓存,GameServer创角时可直接从session验证中取得账号名,
// 分隔符用":"是安全的: session两种生成路径(32位hex/纯数字时间戳回退)都不含":",
// 而accountName即使含":"也不影响按第一个":"的拆分
func NewLoginSession(account *pb.Account) string {
	session := generateRandomSession()
	// 登录session存redis,供玩家登录游戏服时验证用,使登录服和游戏服可以解耦
	_, err := GetRedis().SetEx(context.Background(),
		keyLoginSession(account.GetId()), session+":"+account.GetName(), LoginSessionExpireTime).Result()
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

// VerifyLoginSession 验证登录session
//
// 返回 (验证是否通过, 账号名):
// 缓存值格式为{session}:{accountName},按第一个":"拆分后比对session部分(安全性不变),
// accountName即使含":"也不影响:按第一个":"拆分,剩余整体归accountName;
func VerifyLoginSession(accountId int64, session string) (bool, string) {
	if session == "" {
		return false, ""
	}
	cacheValue, err := GetRedis().Get(context.Background(), keyLoginSession(accountId)).Result()
	if IsRedisError(err) {
		return false, ""
	}
	idx := strings.Index(cacheValue, ":")
	if idx < 0 {
		// 旧格式缓存值(纯session,无账号名): 不再兼容比对待重新登录换取新格式
		return false, ""
	}
	if cacheValue[:idx] != session {
		return false, ""
	}
	return true, cacheValue[idx+1:]
}

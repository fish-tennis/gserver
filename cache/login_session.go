package cache

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"time"
)

func keyLoginSession(accountId int64) string {
	return fmt.Sprintf("ses:%v",accountId)
}

// 新生成一个登录session
func NewLoginSession(account *pb.Account) string {
	session := fmt.Sprintf("%v", time.Now().UnixNano())
	// 登录session存redis,供玩家登录游戏服时验证用,使登录服和游戏服可以解耦
	_,err := GetRedis().SetEX(context.Background(), keyLoginSession(account.GetId()), session, time.Minute*10).Result()
	if IsRedisError(err) {
		gnet.LogError("session err:%v", err)
		return ""
	}
	return session
}

// 验证登录session
func VerifyLoginSession(accountId int64, session string) bool {
	if session == "" {
		return false
	}
	cacheSession,err := GetRedis().Get(context.Background(), keyLoginSession(accountId)).Result()
	if IsRedisError(err) {
		return false
	}
	return cacheSession == session
}
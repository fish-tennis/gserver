package cache

import "github.com/go-redis/redis/v8"

// 缓存直接使用redis,没有抽象一层cache的interface,因为游戏项目现在基本上都用redis来做缓存服务了
var (
	// singleton
	redisClient *redis.ClusterClient
)

func GetRedis() *redis.ClusterClient {
	return redisClient
}

// 初始化redis集群
func NewRedisClient(addrs []string, password string) *redis.ClusterClient {
	redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:addrs,
		Password: password,
	})
	return redisClient
}

// 检查redis返回的error是否是异常
func IsRedisError(redisError error) bool {
	// redis的key不存在,会返回redis.Nil,但是不是我们常规认为的error(异常),所以要忽略redis.Nil
	if redisError != nil && redisError != redis.Nil {
		return true
	}
	return false
}
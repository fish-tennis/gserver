package cache

import "github.com/go-redis/redis/v8"

var (
	// singleton
	redisClient *redis.ClusterClient
)

func GetRedis() *redis.ClusterClient {
	return redisClient
}

func NewRedisClient(addrs []string, password string) *redis.ClusterClient {
	redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:addrs,
		Password: password,
	})
	return redisClient
}
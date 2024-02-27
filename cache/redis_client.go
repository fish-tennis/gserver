package cache

import (
	"github.com/fish-tennis/gentity"
	"github.com/go-redis/redis/v8"
)

var (
	// singleton
	// redis.Cmdable兼容集群模式和单机模式
	_redisClient redis.Cmdable
	_redisCache  gentity.KvCache
)

// 提供KvCache接口,便于更换不同的缓存系统
func Get() gentity.KvCache {
	return _redisCache
}

// 提供redis接口,用于一些redis特有的接口
// Q:其他缓存系统没有的接口,就很难抽象成通用接口了
func GetRedis() redis.Cmdable {
	return _redisClient
}

func NewRedis(addrs []string, userName, password string, isCluster bool) redis.Cmdable {
	var redisCmdable redis.Cmdable
	if isCluster {
		redisCmdable = NewRedisClient(addrs, userName, password)
	} else {
		redisCmdable = NewRedisSingleClient(addrs[0], userName, password)
	}
	_redisCache = gentity.NewRedisCache(redisCmdable)
	return redisCmdable
}

// 初始化redis集群
// 集群不支持事务,但是可以用lua script实现同节点上的原子操作,达到类似事务的效果
func NewRedisClient(addrs []string, userName, password string) redis.Cmdable {
	_redisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:    addrs,
		Username: userName,
		Password: password,
	})
	return _redisClient
}

// 单机模式的redis
func NewRedisSingleClient(addr string, userName, password string) redis.Cmdable {
	_redisClient = redis.NewClient(&redis.Options{
		Addr:     addr,
		Username: userName,
		Password: password,
	})
	return _redisClient
}

// 检查redis返回的error是否是异常
func IsRedisError(redisError error) bool {
	// redis的key不存在,会返回redis.Nil,但是不是我们常规认为的error(异常),所以要忽略redis.Nil
	if redisError != nil && redisError != redis.Nil {
		return true
	}
	return false
}

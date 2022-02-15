package cache

import (
	"google.golang.org/protobuf/proto"
	"time"
)

// 常用的kv缓存接口
type KvCache interface {
	Get(key string) (string,error)
	Set(key string, value interface{}, expiration time.Duration) error
	Del(key string) error

	GetMap(key string,m interface{}) error
	SetMap(key string, m interface{}) error
	SetMapField(key,fieldName string, value interface{}) (isNewField bool, err error)
	DelMapField(key string, fields ...string) error

	GetProto(key string, value proto.Message) error
	SetProto(key string, value proto.Message, expiration time.Duration) error
}
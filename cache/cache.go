package cache

import (
	"google.golang.org/protobuf/proto"
	"time"
)

// 常用的kv缓存接口
type KvCache interface {
	Get(key string) (string, error)
	// value如果是proto.Message,会先进行序列化
	Set(key string, value interface{}, expiration time.Duration) error
	Del(key ...string) error

	Type(key string) (string, error)

	// 缓存数据加载到map
	// m必须是一个类型明确有效的map,且key类型只能是int或string,value类型只能是int或string或proto.Message
	GetMap(key string, m interface{}) error

	// map数据缓存
	// m必须是一个类型明确有效的map,且key类型只能是int或string,value类型只能是int或string或proto.Message
	// NOTE:批量写入数据,并不会删除之前缓存的数据
	SetMap(key string, m interface{}) error

	// 缓存map的一项
	SetMapField(key, fieldName string, value interface{}) (isNewField bool, err error)

	// 删除map的项
	DelMapField(key string, fields ...string) error

	// 缓存数据加载到proto.Message
	GetProto(key string, value proto.Message) error

	// 缓存proto.Message
	SetProto(key string, value proto.Message, expiration time.Duration) error
}

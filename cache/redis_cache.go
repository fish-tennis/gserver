package cache

import (
	"context"
	"errors"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/util"
	"github.com/go-redis/redis/v8"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strconv"
	"time"
)

// https://github.com/uber-go/guide/blob/master/style.md#verify-interface-compliance
var _ KvCache = (*RedisCache)(nil)

type RedisCache struct {
	redisClient redis.Cmdable
}

func NewRedisCache(redisClient redis.Cmdable) *RedisCache {
	return &RedisCache{
		redisClient: redisClient,
	}
}

func (this *RedisCache) Get(key string) (string, error) {
	return this.redisClient.Get(context.Background(), key).Result()
}

func (this *RedisCache) Set(key string, value interface{}, expiration time.Duration) error {
	// 如果是proto,自动转换成[]byte
	if protoMessage,ok := value.(proto.Message); ok {
		bytes,protoErr := proto.Marshal(protoMessage)
		if protoErr != nil {
			return protoErr
		}
		_,err := this.redisClient.Set(context.Background(), key, bytes, expiration).Result()
		return err
	}
	_,err := this.redisClient.Set(context.Background(), key, value, expiration).Result()
	return err
}

func (this *RedisCache) Del(key string) error {
	_,err := this.redisClient.Del(context.Background(), key).Result()
	return err
}

// redis hash -> map
func (this *RedisCache) GetMap(key string, m interface{}) error {
	if m == nil {
		return errors.New("map must valid")
	}
	strMap, err := this.redisClient.HGetAll(context.Background(), key).Result()
	if IsRedisError(err) {
		return err
	}
	val := reflect.ValueOf(m)
	if val.Kind() != reflect.Map {
		return errors.New("unsupport type")
	}
	typ := reflect.TypeOf(m)
	keyType := typ.Key()
	valType := typ.Elem()
	for k,v := range strMap {
		realKey := convertStringToRealType(keyType, k)
		realValue := convertStringToRealType(valType, v)
		val.SetMapIndex(reflect.ValueOf(realKey), reflect.ValueOf(realValue))
	}
	return nil
}

// map -> redis hash
func (this *RedisCache) SetMap(k string, m interface{}) error {
	cacheData := make(map[string]interface{})
	val := reflect.ValueOf(m)
	it := val.MapRange()
	for it.Next() {
		key := convertValueToString(it.Key())
		value := convertValueToStringOrInterface(it.Value())
		cacheData[key] = value
	}
	_,err := this.redisClient.HSet(context.Background(), k, cacheData).Result()
	return err
}

func (this *RedisCache) SetMapField(key,fieldName string, value interface{}) (isNewField bool, err error) {
	ret,redisError := this.redisClient.HSet(context.Background(), key, fieldName, value).Result()
	return ret==1,redisError
}

func (this *RedisCache) DelMapField(key string, fields ...string) error {
	_,err := this.redisClient.HDel(context.Background(), key, fields...).Result()
	return err
}

func (this *RedisCache) GetProto(key string, value proto.Message) error {
	str,err := this.redisClient.Get(context.Background(), key).Result()
	// 不存在的key或者空数据,直接跳过,防止错误的覆盖
	if err == redis.Nil || len(str) == 0 {
		return nil
	}
	if err != nil {
		return err
	}
	err = proto.Unmarshal([]byte(str), value)
	return err
}

func (this *RedisCache) SetProto(key string, value proto.Message, expiration time.Duration) error {
	bytes,protoErr := proto.Marshal(value)
	if protoErr != nil {
		return protoErr
	}
	_,err := this.redisClient.Set(context.Background(), key, bytes, expiration).Result()
	return err
}

func convertValueToString(val reflect.Value) string {
	switch val.Kind() {
	case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
		return strconv.Itoa(int(val.Int()))
	case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
		return strconv.FormatUint(val.Uint(), 10)
	case reflect.String:
		return val.String()
	default:
		logger.Error("unsupport type:%v",val.Kind())
		return ""
	}
}

func convertValueToStringOrInterface(val reflect.Value) interface{} {
	switch val.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
	reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
	reflect.String:
		return convertValueToString(val)
	case reflect.Interface, reflect.Ptr:
		if !val.IsNil() {
			i := val.Interface()
			if protoMessage, ok := i.(proto.Message); ok {
				bytes, protoErr := proto.Marshal(protoMessage)
				if protoErr != nil {
					return protoErr
				}
				return bytes
			}
		}
	default:
		logger.Error("unsupport type:%v",val.Kind())
		return nil
	}
	return nil
}

func convertStringToRealType(typ reflect.Type, v string) interface{} {
	switch typ.Kind() {
	case reflect.Int:
		return util.Atoi(v)
	case reflect.Int8:
		return int8(util.Atoi(v))
	case reflect.Int16:
		return int16(util.Atoi(v))
	case reflect.Int32:
		return int32(util.Atoi(v))
	case reflect.Int64:
		return util.Atoi64(v)
	case reflect.String:
		return v
	case reflect.Interface,reflect.Ptr:
		newProto := reflect.New(typ.Elem())
		if protoMessage,ok := newProto.Interface().(proto.Message); ok {
			protoErr := proto.Unmarshal([]byte(v), protoMessage)
			if protoErr != nil {
				return protoErr
			}
			return protoMessage
		}
	default:
		logger.Error("unsupport type:%v",typ.Kind())
		return nil
	}
	return nil
}
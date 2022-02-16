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
		var realKey interface{}
		switch keyType.Kind() {
		case reflect.Int:
			realKey = util.Atoi(k)
		case reflect.Int8:
			realKey = int8(util.Atoi(k))
		case reflect.Int16:
			realKey = int16(util.Atoi(k))
		case reflect.Int32:
			realKey = int32(util.Atoi(k))
		case reflect.Int64:
			realKey = util.Atoi64(k)
		case reflect.String:
			realKey = k
		default:
			return errors.New("unsupport key type")
		}
		var value interface{}
		switch valType.Kind() {
		case reflect.Int:
			value = util.Atoi(k)
		case reflect.Int8:
			value = int8(util.Atoi(k))
		case reflect.Int16:
			value = int16(util.Atoi(k))
		case reflect.Int32:
			value = int32(util.Atoi(k))
		case reflect.Int64:
			value = util.Atoi64(k)
		case reflect.String:
			value = v
		case reflect.Interface,reflect.Ptr:
			newProto := reflect.New(valType)
			if protoMessage,ok := newProto.Interface().(proto.Message); ok {
				protoErr := proto.Unmarshal([]byte(v), protoMessage)
				if protoErr != nil {
					return protoErr
				}
				value = protoMessage
			}
		default:
			return errors.New("unsupport value type")
		}
		val.SetMapIndex(reflect.ValueOf(realKey), reflect.ValueOf(value))
	}
	return nil
}

// map -> redis hash
func (this *RedisCache) SetMap(k string, m interface{}) error {
	cacheData := make(map[string]interface{})
	val := reflect.ValueOf(m)
	it := val.MapRange()
	for it.Next() {
		var key string
		var value interface{}
		switch it.Key().Kind() {
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
			key = strconv.Itoa(int(it.Key().Int()))
		case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
			key = strconv.FormatUint(it.Key().Uint(), 10)
		case reflect.String:
			key = it.Key().String()
		default:
			return errors.New("unsupport key type")
		}
		switch it.Value().Kind() {
		case reflect.Int,reflect.Int8,reflect.Int16,reflect.Int32,reflect.Int64:
			value = strconv.Itoa(int(it.Value().Int()))
		case reflect.Uint,reflect.Uint8,reflect.Uint16,reflect.Uint32,reflect.Uint64:
			value = strconv.FormatUint(it.Key().Uint(), 10)
		case reflect.String:
			value = it.Value().String()
		case reflect.Interface,reflect.Ptr:
			i := it.Value().Interface()
			if protoMessage,ok := i.(proto.Message); ok {
				bytes,protoErr := proto.Marshal(protoMessage)
				if protoErr != nil {
					return protoErr
				}
				value = bytes
			}
		default:
			logger.Error("unsupport value type:%v",it.Value().Kind())
			return errors.New("unsupport value type")
		}
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
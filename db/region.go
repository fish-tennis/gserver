package db

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// LoadAllRegions 从MongoDB global集合加载全部区服数据
//
// 区服数据以map格式存储在global集合中,文档结构为 {Key:"Regions", Value: map[string]bson.M},
// 其中Value的key为区服id字符串(如"1"),value为bson.M形式的区服字段(Id/Name/Status/时间戳)。
// 返回 map[int32]*pb.Region,key为区服id。
//
// 为什么文档不存在(ErrNoDocuments)时返回空map而非错误:
// Regions文档由LoginServer首次启动初始化或GM后台新增首个区服时才创建,
// GameServer可能先于首次初始化启动,此时库中没有任何区服是合法状态(而非数据异常),
// 若当作错误会导致GameServer启动失败,所以统一按"无区服"处理返回空map。
func LoadAllRegions() (map[int32]*pb.Region, error) {
	regions := make(map[int32]*pb.Region)
	mongoCol := GetGlobalDb().(*gentity.MongoCollection)
	col := mongoCol.GetCollection()
	filter := bson.D{{Key: GlobalDbKeyName, Value: RegionsKeyName}}
	result := col.FindOne(context.Background(), filter)
	if result.Err() != nil {
		if errors.Is(result.Err(), mongo.ErrNoDocuments) {
			return regions, nil
		}
		return nil, fmt.Errorf("query regions failed: %w", result.Err())
	}
	var doc bson.M
	if err := result.Decode(&doc); err != nil {
		return nil, fmt.Errorf("decode regions doc failed: %w", err)
	}
	valueRaw, ok := doc[GlobalDbValueName]
	if !ok {
		// Value字段缺失与文档不存在语义一致,同样视为无区服数据而非格式错误
		return regions, nil
	}
	// MongoDB的BSON map的key是字符串类型,需要先序列化再反序列化来完成类型转换
	var rawMap map[string]bson.M
	bsonBytes, err := bson.Marshal(valueRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal regions value failed: %w", err)
	}
	if err := bson.Unmarshal(bsonBytes, &rawMap); err != nil {
		return nil, fmt.Errorf("unmarshal regions value failed: %w", err)
	}
	for k, v := range rawMap {
		// key为区服id字符串,解析为int32;解析失败或非正数的key视为脏数据跳过
		regionId64, err := strconv.ParseInt(k, 10, 32)
		if err != nil || regionId64 <= 0 {
			slog.Warn("LoadAllRegions invalid region key, skip", "key", k)
			continue
		}
		regionId := int32(regionId64)
		region := &pb.Region{}
		bsonBytes2, err := bson.Marshal(v)
		if err != nil {
			// 单条脏数据跳过并告警,不影响其余区服加载
			slog.Warn("LoadAllRegions marshal region failed, skip", "regionId", regionId, "error", err)
			continue
		}
		if err := bson.Unmarshal(bsonBytes2, region); err != nil {
			slog.Warn("LoadAllRegions unmarshal region failed, skip", "regionId", regionId, "error", err)
			continue
		}
		regions[regionId] = region
	}
	return regions, nil
}

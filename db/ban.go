package db

import (
	"fmt"
	"time"

	"github.com/fish-tennis/gserver/pb"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	BanTargetTypeAccount int32 = 1 // 账号封禁
	BanTargetTypePlayer  int32 = 2 // 玩家封禁
)

// banKey 构造封禁记录的复合key,格式: "{TargetType}_{TargetId}"
// AccountId和PlayerId值域可能重叠,用复合key区分
func banKey(targetType int32, targetId int64) string {
	return fmt.Sprintf("%d_%d", targetType, targetId)
}

// IsBanned 检查目标是否处于封禁状态
func IsBanned(targetType int32, targetId int64) bool {
	return GetBanRecord(targetType, targetId) != nil
}

// GetBanRecord 查询目标的封禁记录,未封禁或已过期返回nil
func GetBanRecord(targetType int32, targetId int64) *pb.BanRecord {
	record := &pb.BanRecord{}
	found, err := GetDbMgr().GetEntityDb(BanDbName).FindEntityById(banKey(targetType, targetId), record)
	if err != nil || !found {
		return nil
	}
	if record.Duration == 0 {
		return record // 永久封禁
	}
	if record.BanTime+record.Duration > time.Now().Unix() {
		return record // 封禁期内
	}
	return nil // 封禁已过期
}

// SaveBanRecord 保存封禁记录到MongoDB
// _id使用复合key "{TargetType}_{TargetId}",避免accountId和playerId冲突
func SaveBanRecord(record *pb.BanRecord) error {
	key := banKey(record.TargetType, record.TargetId)
	banData := map[string]any{
		UniqueIdName: key,
		"TargetId":   record.TargetId,
		"TargetType": record.TargetType,
		"BanTime":    record.BanTime,
		"Duration":   record.Duration,
		"Reason":     record.Reason,
	}
	err, isDuplicateKey := GetDbMgr().GetEntityDb(BanDbName).InsertEntity(key, banData)
	if err != nil && isDuplicateKey {
		// 记录已存在(重复封禁),执行更新
		// NOTE:SaveEntity把第二个参数直接作为update文档,必须携带$set等原子操作符,
		// 裸map会被MongoDB拒绝:update document must contain key beginning with '$'
		updateData := map[string]any{
			"TargetId":   record.TargetId,
			"TargetType": record.TargetType,
			"BanTime":    record.BanTime,
			"Duration":   record.Duration,
			"Reason":     record.Reason,
		}
		return GetDbMgr().GetEntityDb(BanDbName).SaveEntity(key, bson.M{"$set": updateData})
	}
	return err
}

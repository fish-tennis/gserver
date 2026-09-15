package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/fish-tennis/gentity"
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

// banRecordActive 判断封禁记录是否处于生效状态(在内存中判定,不查库)
// Duration==0为永久封禁;限期封禁超过BanTime+Duration视为已过期
func banRecordActive(record *pb.BanRecord) bool {
	if record.Duration == 0 {
		return true // 永久封禁
	}
	return record.BanTime+record.Duration > time.Now().Unix()
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
	if !banRecordActive(record) {
		return nil // 封禁已过期
	}
	return record
}

// GetBanRecordsForLogin 一次MongoDB往返同时查询账号级与玩家级封禁记录
// 返回(nil表示该维度未封禁或已过期)
func GetBanRecordsForLogin(accountId int64, playerId int64) (accountBan *pb.BanRecord, playerBan *pb.BanRecord) {
	accountKey := banKey(BanTargetTypeAccount, accountId)
	playerKey := banKey(BanTargetTypePlayer, playerId)
	// 与gentity运行时CRUD(FindEntityById的opCtx)相同的超时口径:
	// gentity的opCtx未导出,用导出的GetMongoOpTimeout对齐(默认60秒,支持运行时调整)。
	// 不能用context.Background():MongoDB静默卡死(对端假死/静默断连)时Find/cursor.All
	// 会永久阻塞,占死DB worker并拖垮同hash槽的所有进游/重连请求
	ctx, cancel := context.WithTimeout(context.Background(), gentity.GetMongoOpTimeout())
	defer cancel()
	mongoCol, ok := GetDbMgr().GetEntityDb(BanDbName).(*gentity.MongoCollection)
	if !ok {
		slog.Error("GetBanRecordsForLogin: unsupported ban db type", "accountId", accountId, "playerId", playerId)
		return nil, nil
	}
	cursor, err := mongoCol.GetCollection().Find(
		ctx,
		bson.D{{Key: UniqueIdName, Value: bson.D{
			{Key: "$in", Value: []string{accountKey, playerKey}},
		}}},
	)
	if err != nil {
		return nil, nil
	}
	var records []*pb.BanRecord
	if err = cursor.All(ctx, &records); err != nil {
		return nil, nil
	}
	for _, record := range records {
		if !banRecordActive(record) {
			continue
		}
		// 按记录自身的TargetType分发到对应维度,而不是按数组顺序赋值
		switch record.GetTargetType() {
		case BanTargetTypeAccount:
			accountBan = record
		case BanTargetTypePlayer:
			playerBan = record
		}
	}
	return accountBan, playerBan
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

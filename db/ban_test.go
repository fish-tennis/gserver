package db

import (
	"testing"
	"time"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/pb"
)

// initBanTestDb 为封禁测试初始化MongoDB(仅注册ban集合)
// 本地MongoDB不可用时跳过;DbMgr已被同包其他用例初始化但未注册ban集合时补注册
// (与account_player_test.go处理同包共享DbMgr的方式一致)
func initBanTestDb(t *testing.T) {
	t.Helper()
	if GetDbMgr() == nil {
		mongoDb := gentity.NewMongoDb("mongodb://127.0.0.1:27017", "eat_test_ban")
		RegisterBanDb(mongoDb)
		if !mongoDb.Connect() {
			t.Skip("本地MongoDB不可用,跳过封禁测试")
		}
		SetDbMgr(mongoDb)
		return
	}
	if GetDbMgr().GetEntityDb(BanDbName) == nil {
		RegisterBanDb(GetDbMgr().(*gentity.MongoDb))
	}
}

// TestBanRecordSaveAndQuery 钉住封禁记录的保存与查询语义
// 回归保护:duplicate分支的update曾传裸map(无$set),被MongoDB拒绝,
// 导致重复封禁/修改封禁/续期100%失败(update document must contain key beginning with '$')
func TestBanRecordSaveAndQuery(t *testing.T) {
	initBanTestDb(t)
	if GetDbMgr() == nil {
		return // 本地MongoDB不可用时initBanTestDb内部已Skip
	}
	// 用纳秒级唯一targetId隔离用例数据
	targetId := time.Now().UnixNano()

	// 未封禁时查询返回nil
	if GetBanRecord(BanTargetTypeAccount, targetId) != nil {
		t.Fatal("未封禁应返回nil")
	}
	// 首次封禁:限期100秒
	if err := SaveBanRecord(&pb.BanRecord{
		TargetType: BanTargetTypeAccount, TargetId: targetId,
		BanTime: time.Now().Unix(), Duration: 100, Reason: "first",
	}); err != nil {
		t.Fatalf("首次保存: %v", err)
	}
	record := GetBanRecord(BanTargetTypeAccount, targetId)
	if record == nil || record.Duration != 100 || record.Reason != "first" {
		t.Fatalf("首次保存后查询异常: %+v", record)
	}
	// 重复封禁(更新已有记录):续期为200秒——回归点,曾在此分支100%报错
	if err := SaveBanRecord(&pb.BanRecord{
		TargetType: BanTargetTypeAccount, TargetId: targetId,
		BanTime: time.Now().Unix(), Duration: 200, Reason: "update",
	}); err != nil {
		t.Fatalf("重复保存(duplicate分支)报错: %v", err)
	}
	record = GetBanRecord(BanTargetTypeAccount, targetId)
	if record == nil || record.Duration != 200 || record.Reason != "update" {
		t.Fatalf("重复保存后查询应看到更新值: %+v", record)
	}
	// 已过期(限期已过)返回nil
	if err := SaveBanRecord(&pb.BanRecord{
		TargetType: BanTargetTypePlayer, TargetId: targetId,
		BanTime: time.Now().Unix() - 300, Duration: 100, Reason: "expired",
	}); err != nil {
		t.Fatalf("保存过期记录: %v", err)
	}
	if GetBanRecord(BanTargetTypePlayer, targetId) != nil {
		t.Fatal("已过期封禁应返回nil")
	}
}

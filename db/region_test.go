package db

import (
	"context"
	"testing"

	"github.com/fish-tennis/gentity"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// initRegionTestDb 为区服加载测试初始化MongoDB(仅注册global集合)
// 本地MongoDB不可用时跳过;DbMgr已被同包其他用例初始化但未注册global集合时补注册
// (与account_player_test.go处理同包共享DbMgr的方式一致)
func initRegionTestDb(t *testing.T) {
	t.Helper()
	if GetDbMgr() == nil {
		mongoDb := gentity.NewMongoDb("mongodb://127.0.0.1:27017", "eat_test_region")
		RegisterGlobalEntityDb(mongoDb)
		if !mongoDb.Connect() {
			t.Skip("本地MongoDB不可用,跳过区服加载测试")
		}
		SetDbMgr(mongoDb)
		return
	}
	if GetDbMgr().GetEntityDb(GlobalDbName) == nil {
		RegisterGlobalEntityDb(GetDbMgr().(*gentity.MongoDb))
	}
}

// TestLoadAllRegions 钉住LoadAllRegions的解析语义:
// 正常key加载、脏key(非数字/非正数)跳过不报错、文档不存在返回空map
// 区服数据是internal内存快照的唯一数据源,解析错误会导致区服在GameServer上"消失",
// 进游/建角的区服校验会把玩家全部拒之门外,必须有测试钉住
func TestLoadAllRegions(t *testing.T) {
	initRegionTestDb(t)
	if GetDbMgr() == nil {
		return // 本地MongoDB不可用时initRegionTestDb内部已Skip
	}
	ctx := context.Background()
	col := GetGlobalDb().(*gentity.MongoCollection).GetCollection()
	// 清场后写入测试数据:2个合法区服+3个脏key(非数字/0/负数)
	_, _ = col.DeleteOne(ctx, bson.M{GlobalDbKeyName: RegionsKeyName})
	_, err := col.InsertOne(ctx, bson.M{
		GlobalDbKeyName: RegionsKeyName,
		GlobalDbValueName: bson.M{
			"1": bson.M{"Id": 1, "Name": "r1", "Status": 1},
			"2": bson.M{"Id": 2, "Name": "r2", "Status": 2},
			// 脏key:Sscanf部分解析("12abc"会解析成12),必须被严格解析跳过
			"12abc": bson.M{"Id": 12, "Name": "dirty", "Status": 1},
			"0":     bson.M{"Id": 0, "Name": "zero", "Status": 1},
			"-1":    bson.M{"Id": -1, "Name": "neg", "Status": 1},
		},
	})
	if err != nil {
		t.Fatalf("插入Regions文档: %v", err)
	}
	regions, err := LoadAllRegions()
	if err != nil {
		t.Fatalf("LoadAllRegions: %v", err)
	}
	if len(regions) != 2 {
		t.Fatalf("应只加载2个合法区服(脏key被跳过): got %v", regions)
	}
	if regions[1] == nil || regions[1].Name != "r1" || regions[2] == nil || regions[2].Name != "r2" {
		t.Fatalf("区服数据异常: %+v", regions)
	}
	if _, ok := regions[12]; ok {
		t.Fatal("脏key(12abc)不应被部分解析成12加载")
	}
	// 文档不存在:返回空map而非错误(GameServer先于首次初始化启动的合法状态)
	if _, err = col.DeleteOne(ctx, bson.M{GlobalDbKeyName: RegionsKeyName}); err != nil {
		t.Fatalf("清理: %v", err)
	}
	regions, err = LoadAllRegions()
	if err != nil || len(regions) != 0 {
		t.Fatalf("文档不存在应返回空map无错误: regions=%v err=%v", regions, err)
	}
}

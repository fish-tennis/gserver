package gameplayer

import (
	"context"
	"fmt"
	"github.com/fish-tennis/gserver/db"
	"github.com/fish-tennis/gserver/db/mongodb"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"go.mongodb.org/mongo-driver/bson"
	"testing"
)

func TestSaveable(t *testing.T) {
	util.InitIdGenerator(1)
	InitPlayerComponentMap()

	player := CreateTempPlayer(1, 1)
	// 明文保存的proto
	baseInfo := player.GetComponentByName("BaseInfo").(*BaseInfo)
	baseInfo.IncExp(1001)
	saveData,err := internal.GetComponentSaveData(baseInfo)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 序列化保存的proto
	money := player.GetComponentByName("Money").(*Money)
	money.IncCoin(10)
	money.IncDiamond(100)
	saveData,err = internal.GetComponentSaveData(money)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// 基础类型的map
	countBag := player.GetComponentByName("BagCountItem").(*BagCountItem)
	for i := 0; i < 3; i++ {
		countBag.AddItem(int32(i+1), int32((i+1)*10))
	}
	saveData,err = internal.GetComponentSaveData(countBag)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是proto的map
	uniqueBag := player.GetComponentByName("BagUniqueItem").(*BagUniqueItem)
	for i := 0; i < 3; i++ {
		uniqueBag.AddUniqueItem(&pb.UniqueItem{
			UniqueId: util.GenUniqueId(),
			CfgId: int32(i+1),
		})
	}
	saveData,err = internal.GetComponentSaveData(uniqueBag)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))

	// value是子模块的组合
	quest := player.GetComponentByName("Quest").(*Quest)
	quest.Quests.Add(&pb.QuestData{
		CfgId: 1,
		Progress: 0,
	})
	quest.Quests.Add(&pb.QuestData{
		CfgId: 2,
		Progress: 1,
	})
	quest.Finished.Add(3)
	quest.Finished.Add(4)
	saveData,err = internal.GetComponentSaveData(quest)
	if err != nil {
		t.Error(err)
	}
	t.Log(fmt.Sprintf("%v", saveData))
}

func TestLoadAndSaveData(t *testing.T) {
	util.InitIdGenerator(1)
	InitPlayerComponentMap()

	mongoDb := mongodb.NewMongoDb("mongodb://localhost:27017","testdb")
	mongoDb.RegisterPlayerPb("player", "id", "name", "accountid", "regionid")
	if !mongoDb.Connect() {
		t.Fatal("connect db error")
	}
	db.SetDbMgr(mongoDb)
	playerData := &pb.PlayerData{}
	hasData,err := db.GetPlayerDb().FindPlayerByAccountId(209731441704042496, 1, playerData)
	if err != nil {
		t.Fatalf("%v", err )
	}
	if !hasData {
		t.Fatal("not find player data")
	}
	player := CreatePlayerFromData(playerData)

	player.id = 1234
	player.accountId = 123456
	player.name = "test1234"
	newPlayerSaveData := make(map[string]interface{})
	newPlayerSaveData["id"] = player.id
	newPlayerSaveData["name"] = player.GetName()
	newPlayerSaveData["accountid"] = player.GetAccountId()
	newPlayerSaveData["regionid"] = player.GetRegionId()
	internal.GetEntitySaveData(player, newPlayerSaveData)
	t.Logf("%v", newPlayerSaveData)
	mongoDb.GetMongoDatabase().Collection("player").DeleteOne(context.Background(), bson.D{{"id",player.id}})
	insertErr,isDuplicateKey := db.GetPlayerDb().InsertEntity(player.id, newPlayerSaveData)
	if insertErr != nil {
		if isDuplicateKey {
			t.Error("DuplicateKey")
		}
		t.Fatalf("%v", insertErr )
	}
}
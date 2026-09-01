package db

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/fish-tennis/gentity"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// TestGenAccountPlayerKey 测试注册表复合key构造函数
// key是注册表的_id,构造错误会导致不同账号/区服的注册位互相覆盖(误占或漏占),
// 或同一账号区服组合生成不同key(防重失效)
func TestGenAccountPlayerKey(t *testing.T) {
	tests := []struct {
		name      string
		accountId int64
		regionId  int32
		want      string
	}{
		{name: "常规账号区服", accountId: 12345, regionId: 1, want: "12345_1"},
		{name: "相同id不同含义不碰撞_账号视角", accountId: 100, regionId: 2, want: "100_2"},
		{name: "相同id不同含义不碰撞_区服视角", accountId: 200, regionId: 100, want: "200_100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenAccountPlayerKey(tt.accountId, tt.regionId)
			if got != tt.want {
				t.Errorf("GenAccountPlayerKey(%d, %d) = %q, want %q", tt.accountId, tt.regionId, got, tt.want)
			}
		})
	}
}

// TestGenAccountPlayerKey_NoCollision 验证账号与区服数值互换时key不同,
// 以及同账号不同区服/不同账号同区服的key均不冲突
// (这是用字符串"_"分隔而非数字拼接的原因:数字拼接会产生歧义碰撞)
func TestGenAccountPlayerKey_NoCollision(t *testing.T) {
	// (1,100)与(100,1)若用数字拼接可能同值,字符串分隔则必不同
	if GenAccountPlayerKey(1, 100) == GenAccountPlayerKey(100, 1) {
		t.Fatalf("账号与区服数值互换的key不应相同")
	}
	// 同账号不同区服:互不冲突
	if GenAccountPlayerKey(1, 1) == GenAccountPlayerKey(1, 2) {
		t.Fatalf("同账号不同区服的key不应相同")
	}
	// 不同账号同区服:互不冲突
	if GenAccountPlayerKey(1, 1) == GenAccountPlayerKey(2, 1) {
		t.Fatalf("不同账号同区服的key不应相同")
	}
}

// initAccountPlayerTestDb 为注册表测试初始化MongoDB(仅注册player_account集合)
// 本地MongoDB不可用时跳过,避免CI环境因无DB而失败
func initAccountPlayerTestDb(t *testing.T) {
	t.Helper()
	if GetDbMgr() != nil {
		return
	}
	// 使用独立的测试库,避免污染开发数据
	mongoDb := gentity.NewMongoDb("mongodb://127.0.0.1:27017", "eat_test_account_player")
	RegisterAccountPlayerDb(mongoDb)
	if !mongoDb.Connect() {
		t.Skip("本地MongoDB不可用,跳过建角注册表测试")
	}
	SetDbMgr(mongoDb)
}

// TestAccountPlayerMapSemantics 钉住映射表的持久防重与查询语义(建角防重的基础契约)
// (true,nil)=写入成功 / (false,nil)=该账号该区服已有角色 / (false,err)=数据库异常,
// 调用方processCreatePlayerReq按(false,nil)直接返回PlayerAlreadyExist(映射是持久事实,
// _id冲突有且只有一种含义,无需再查player表裁决);
// 若占用被误判为err,并发建角会报DbErr而非PlayerAlreadyExist;
// 若占用被误放行(返回true),防双角色约束直接失效
func TestAccountPlayerMapSemantics(t *testing.T) {
	initAccountPlayerTestDb(t)
	if GetDbMgr() == nil {
		return // 本地MongoDB不可用时initAccountPlayerTestDb内部已Skip
	}
	// 用纳秒级唯一accountId隔离用例数据,避免重复运行互相污染
	accountId := time.Now().UnixNano()
	const regionId = int32(1)

	// 首次写入:必须成功
	ok, err := InsertAccountPlayerMap(accountId, regionId, 100001)
	if err != nil || !ok {
		t.Fatalf("首次写入应成功: ok=%v err=%v", ok, err)
	}
	// 同账号同区服重复写入:占用而非错误(false,nil),E11000必须被正确识别
	ok, err = InsertAccountPlayerMap(accountId, regionId, 100002)
	if err != nil || ok {
		t.Fatalf("重复写入应返回(false,nil): ok=%v err=%v(占用被误判,建角防重语义失效)", ok, err)
	}
	// FindPlayerIdByAccount:查到首次写入的playerId(而非重复写入的100002)
	playerId, err := FindPlayerIdByAccount(accountId, regionId)
	if err != nil || playerId != 100001 {
		t.Fatalf("FindPlayerIdByAccount应返回100001: playerId=%v err=%v", playerId, err)
	}
	// 未建角色的区服:返回0
	playerId, err = FindPlayerIdByAccount(accountId, regionId+9)
	if err != nil || playerId != 0 {
		t.Fatalf("未建角色的区服应返回0: playerId=%v err=%v", playerId, err)
	}
	// 同账号不同区服:互不干扰(一个账号可在多个区服各建1个角色)
	ok, err = InsertAccountPlayerMap(accountId, regionId+1, 100003)
	if err != nil || !ok {
		t.Fatalf("同账号不同区服写入应成功: ok=%v err=%v", ok, err)
	}
	// 按账号查所有区服的角色列表:应恰好2条(登录角色列表入口)
	accounts, err := FindAccountPlayersByAccount(accountId)
	if err != nil || len(accounts) != 2 {
		t.Fatalf("按账号查角色列表应2条: len=%v err=%v", len(accounts), err)
	}
	// 删除后可立即重新写入:建角失败的回滚语义
	if err = DeleteAccountPlayerMap(accountId, regionId); err != nil {
		t.Fatalf("删除映射失败: %v", err)
	}
	playerId, err = FindPlayerIdByAccount(accountId, regionId)
	if err != nil || playerId != 0 {
		t.Fatalf("删除后查询应返回0: playerId=%v err=%v", playerId, err)
	}
	ok, err = InsertAccountPlayerMap(accountId, regionId, 100004)
	if err != nil || !ok {
		t.Fatalf("删除后重新写入应成功: ok=%v err=%v(回滚失效,建角失败后无法重试)", ok, err)
	}
}

// TestInsertAccountPlayerMapConcurrent 并发建角的最终原子防线
// 双端同时建角/请求重试/恶意刷时,多个请求的insert同时到达,
// 映射表_id唯一性是唯一能原子裁决的防线:N个并发写入必须恰好1个成功,
// 出现2个及以上成功即产生双角色(数据完整性事故),出现0个成功则正常建角被误拒
func TestInsertAccountPlayerMapConcurrent(t *testing.T) {
	initAccountPlayerTestDb(t)
	if GetDbMgr() == nil {
		return
	}
	accountId := time.Now().UnixNano()
	const regionId = int32(2)
	const n = 20
	okCh := make(chan bool, n)
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ok, err := InsertAccountPlayerMap(accountId, regionId, 100005)
			okCh <- ok
			errCh <- err
		}()
	}
	wg.Wait()
	close(okCh)
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("并发写入出现错误: %v", err)
		}
	}
	successCount := 0
	for ok := range okCh {
		if ok {
			successCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("并发写入应恰好1个成功,实际%d个(防双角色防线失效)", successCount)
	}
}

// InitPlayerIndexTestDb 为player集合索引测试初始化MongoDB
// 本地MongoDB不可用时跳过;DbMgr已被同包其他用例初始化但未注册player集合时补注册
// (与region_test.go处理同包共享DbMgr的方式一致)
func InitPlayerIndexTestDb(t *testing.T) {
	t.Helper()
	if GetDbMgr() == nil {
		mongoDb := gentity.NewMongoDb("mongodb://127.0.0.1:27017", "eat_test_player_index")
		// 注册方式与生产gameserver.initDb保持一致(RegisterPlayerDb)
		RegisterPlayerDb(mongoDb)
		if !mongoDb.Connect() {
			t.Skip("本地MongoDB不可用,跳过player索引测试")
		}
		SetDbMgr(mongoDb)
		return
	}
	if GetDbMgr().GetEntityDb(PlayerDbName) == nil {
		RegisterPlayerDb(GetDbMgr().(*gentity.MongoDb))
	}
}

// TestEnsurePlayerAccountRegionIndex_CreatesCompoundIndex 钉住90732d72d(玩家表查询优化)
// 引入的player复合索引:该索引服务登录角色列表/建角预检查/GM搜玩家三条高频查询路径,
// GameServer启动时创建失败会直接panic(启动关键路径);
// 通过ListIndexes验证索引键确为{AccountId,RegionId}复合键——字段名改写或
// 索引定义被误改为单列时,登录链路在分片集群上会退化为全shard扫描,必须有测试钉住
func TestEnsurePlayerAccountRegionIndex_CreatesCompoundIndex(t *testing.T) {
	InitPlayerIndexTestDb(t)

	if err := EnsurePlayerAccountRegionIndex(); err != nil {
		t.Fatalf("创建复合索引应成功(GameServer启动失败即panic的路径): %v", err)
	}

	// CreateOne返回成功不代表键定义正确(键名错误也能建成功),必须回读索引定义核对
	cursor, err := GetPlayerDb().(*gentity.MongoCollectionPlayer).GetCollection().
		Indexes().List(context.Background())
	if err != nil {
		t.Fatalf("列出player索引失败: %v", err)
	}
	var specs []bson.M
	if err := cursor.All(context.Background(), &specs); err != nil {
		t.Fatalf("解析player索引列表失败: %v", err)
	}
	for _, spec := range specs {
		keyDoc, ok := spec["key"].(bson.D)
		if !ok || len(keyDoc) != 2 {
			continue
		}
		// 键名与顺序都必须精确匹配:顺序错误(RegionId在前)无法服务
		// FindPlayerIdByAccountId的{AccountId,RegionId}等值查询前缀
		if keyDoc[0].Key == PlayerAccountId && keyDoc[1].Key == PlayerRegionId {
			return
		}
	}
	t.Fatalf("player集合应存在{AccountId,RegionId}复合索引,实际索引定义: %v", specs)
}

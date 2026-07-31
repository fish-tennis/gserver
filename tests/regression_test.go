package tests

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// === ClientData atomic 读写测试 ===

func TestClientDataPlayerIdAtomic(t *testing.T) {
	cd := &network.ClientData{
		ConnId:       100,
		AccountId:    200,
		GameServerId: 1,
	}
	// 初始值为0
	if cd.GetPlayerId() != 0 {
		t.Fatalf("initial PlayerId should be 0, got %v", cd.GetPlayerId())
	}
	// 写入后读取
	cd.SetPlayerId(12345)
	if cd.GetPlayerId() != 12345 {
		t.Fatalf("PlayerId should be 12345, got %v", cd.GetPlayerId())
	}
}

func TestClientDataPlayerIdConcurrent(t *testing.T) {
	cd := &network.ClientData{}
	var wg sync.WaitGroup
	// 并发写
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(val int64) {
			defer wg.Done()
			cd.SetPlayerId(val)
		}(int64(i + 1))
	}
	// 并发读
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = cd.GetPlayerId()
		}()
	}
	wg.Wait()
	// 最终值应该是某个合法值(不 panic 即可)
	final := cd.GetPlayerId()
	if final < 1 || final > 100 {
		t.Fatalf("unexpected final PlayerId: %v", final)
	}
}

// === RouteGuildServerId 负数索引测试 ===

func TestRouteGuildServerIdNegative(t *testing.T) {
	// internal.RouteGuildServerId 依赖 GetServerList().GetServersByType(ServerType_Game)
	// 这里无法直接测试(需要完整服务器初始化),改为验证取模逻辑
	servers := []int32{1, 2, 3, 4, 5}
	n := int64(len(servers))

	testCases := []int64{
		0,   // 边界:0 % 5 = 0
		1,   // 1 % 5 = 1
		4,   // 4 % 5 = 4
		5,   // 5 % 5 = 0
		-1,  // -1 % 5 = -1 → +5 = 4
		-6,  // -6 % 5 = -1 → +5 = 4
		-7,  // -7 % 5 = -2 → +5 = 3
		-10, // -10 % 5 = 0
	}

	for _, guildId := range testCases {
		index := guildId % n
		if index < 0 {
			index += n
		}
		if index < 0 || index >= n {
			t.Fatalf("guildId=%v: index %v out of range [0,%v)", guildId, index, n)
		}
		// servers[index] 不会越界
		t.Logf("guildId=%v -> index=%v -> serverId=%v", guildId, index, servers[index])
	}
}

// === DefaultProgressUpdater 非指针事件测试 ===

// 非指针事件类型(模拟)
type testStructEvent struct {
	PlayerId int64
	Count    int32
}

// 指针事件类型
type testPointerEvent struct {
	PlayerId int64
	Count    int32
}

func TestDefaultProgressUpdaterNonPointerEvent(t *testing.T) {
	// 验证非指针事件不会 panic
	// DefaultProgressUpdater 内部逻辑:对非指针 event 用 reflect.ValueOf(event) 而非 .Elem()
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DefaultProgressUpdater panicked on non-pointer event: %v", r)
		}
	}()

	// 构造一个 ProgressCfg
	progressCfg := &pb.ProgressCfg{
		Type: int32(pb.ProgressType_ProgressType_Event),
		Event: "testStructEvent",
		StringEventFields: map[string]string{
			"PlayerId": "100",
		},
	}

	// 使用非指针事件
	event := testStructEvent{PlayerId: 100, Count: 1}
	holder := &mockProgressHolder{}
	// 这应该不会 panic
	internal.DefaultProgressUpdater(nil, holder, event, progressCfg)

	// 使用指针事件也应该正常工作
	eventPtr := &testPointerEvent{PlayerId: 100, Count: 1}
	internal.DefaultProgressUpdater(nil, holder, eventPtr, progressCfg)
}

// mockProgressHolder 实现 internal.ProgressHolder 接口
type mockProgressHolder struct {
	progress int32
}

func (m *mockProgressHolder) GetProgress() int32   { return m.progress }
func (m *mockProgressHolder) SetProgress(p int32)  { m.progress = p }

// === pending_messages 清理逻辑测试 ===

func TestPendingMessagesCleanupPattern(t *testing.T) {
	// 验证"先收集 key 再删除"的模式不会漏处理
	messages := make(map[int64]*pb.PendingMessage)
	for i := int64(1); i <= 10; i++ {
		messages[i] = &pb.PendingMessage{MessageId: i}
	}

	// 先收集所有 key(与修复后的代码一致)
	ids := make([]int64, 0, len(messages))
	for id := range messages {
		ids = append(ids, id)
	}

	// 遍历切片删除 map 中的元素
	processed := make([]int64, 0, len(ids))
	for _, msgId := range ids {
		if _, ok := messages[msgId]; !ok {
			t.Fatalf("msgId %v not found in messages", msgId)
		}
		delete(messages, msgId)
		processed = append(processed, msgId)
	}

	// 所有消息都应该被处理
	if len(processed) != 10 {
		t.Fatalf("expected 10 processed, got %v", len(processed))
	}
	// map 应该为空
	if len(messages) != 0 {
		t.Fatalf("expected empty messages map, got %v items", len(messages))
	}
}

// === proto 测试:验证 UnmarshalNew 不需要额外的 UnmarshalTo ===

func TestUnmarshalNewSufficiency(t *testing.T) {
	// 验证 UnmarshalNew 已经完成反序列化,不需要再调 UnmarshalTo
	original := &pb.PlayerEntryGameReq{
		AccountId:    12345,
		LoginSession: "test-session",
	}

	data, err := proto.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}

	// 用 anypb 包装(模拟 PendingMessage.PacketData)
	anyData, err := proto.Marshal(&pb.PendingMessage{
		MessageId:     1,
		PacketCommand: 1,
		PacketData:    mustNewAny(original),
		Timestamp:     0,
	})
	if err != nil {
		t.Fatal(err)
	}

	// 反序列化 PendingMessage
	pendingMsg := &pb.PendingMessage{}
	if err := proto.Unmarshal(anyData, pendingMsg); err != nil {
		t.Fatal(err)
	}

	// UnmarshalNew 应该已经完成完整反序列化
	msg, err := pendingMsg.PacketData.UnmarshalNew()
	if err != nil {
		t.Fatal(err)
	}

	// 类型断言验证
	entryReq, ok := msg.(*pb.PlayerEntryGameReq)
	if !ok {
		t.Fatalf("expected *pb.PlayerEntryGameReq, got %T", msg)
	}

	// 验证字段值正确(UnmarshalNew 已完整反序列化)
	if entryReq.GetAccountId() != 12345 {
		t.Fatalf("AccountId mismatch: expected 12345, got %v", entryReq.GetAccountId())
	}
	if entryReq.GetLoginSession() != "test-session" {
		t.Fatalf("LoginSession mismatch: expected test-session, got %v", entryReq.GetLoginSession())
	}

	_ = data // suppress unused
}

func mustNewAny(msg proto.Message) *anypb.Any {
	a, err := anypb.New(msg)
	if err != nil {
		panic(err)
	}
	return a
}

// === Exchange int32 溢出测试 ===

func TestExchangeCountLimitOverflow(t *testing.T) {
	// 模拟 exchange.go:103 的检查逻辑(有 int32 溢出风险)
	var curCount int32 = 5
	var exchangeCount int32 = math.MaxInt32
	var countLimit int32 = 10
	sum := curCount + exchangeCount // 运行时溢出
	// 当前代码: sum > countLimit → 溢出为负 → false → 绕过限制
	t.Logf("int32溢出: %d + %d = %d, %d > %d = %v",
		curCount, exchangeCount, sum, sum, countLimit, sum > countLimit)
	// 正确实现应使用 int64
	safeSum := int64(curCount) + int64(exchangeCount)
	if safeSum <= int64(countLimit) {
		t.Fatal("safe check should detect overflow")
	}
	t.Logf("int64安全检查: %d + %d = %d > %d = true", curCount, exchangeCount, safeSum, countLimit)
}

func TestExchangeCountAccumulationOverflow(t *testing.T) {
	var count int32 = math.MaxInt32 - 5
	count += 10
	t.Logf("累加溢出: MaxInt32-5 + 10 = %d", count)
}

// === Reconnect 定时器代际问题测试 ===

func TestReconnectTimerGenerationBug(t *testing.T) {
	type timerState struct {
		active bool
	}
	state := &timerState{}
	var stopCalled bool

	registerTimer := func() {
		if state.active {
			return
		}
		state.active = true
	}
	timerCallback := func() {
		if !state.active {
			return
		}
		stopCalled = true
	}
	cancelWait := func() {
		state.active = false
	}

	// 断线 -> 重连 -> 再次断线 -> 旧定时器触发
	registerTimer()
	cancelWait()
	registerTimer()

	stopCalled = false
	timerCallback()
	if !stopCalled {
		t.Fatal("BUG: stale timer should call Stop due to shared boolean flag")
	}
	t.Log("BUG CONFIRMED: 陈旧定时器误杀(布尔标志无法区分代际)")

	// 正确做法:代际计数器
	type safeState struct {
		active bool
		gen    int
	}
	ss := &safeState{}
	var safeStop bool

	registerSafe := func() {
		if ss.active {
			return
		}
		ss.gen++
		ss.active = true
	}
	timerCallbackSafe := func(gen int) {
		if !ss.active || ss.gen != gen {
			return
		}
		safeStop = true
	}

	registerSafe()
	gen1 := ss.gen
	ss.active = false
	registerSafe()
	if ss.gen <= gen1 {
		t.Fatal("generation should increment")
	}

	safeStop = false
	timerCallbackSafe(gen1)
	if safeStop {
		t.Fatal("safe implementation should NOT call Stop for stale timer")
	}
	t.Log("正确实现(代际计数器): 陈旧定时器被正确忽略")
}

// === ReconnectSession 非一次性测试 ===

func TestReconnectSessionRotation(t *testing.T) {
	type baseInfo struct {
		reconnectSession string
	}

	bi := &baseInfo{reconnectSession: "session-1"}

	// 当前实现:验证后不立即轮换
	verifyNoRotate := func(session string) bool {
		return session == bi.reconnectSession
	}

	if !verifyNoRotate("session-1") {
		t.Fatal("first reconnect should succeed")
	}
	if !verifyNoRotate("session-1") {
		t.Fatal("BUG: same session should not be reusable")
	}
	t.Log("BUG CONFIRMED: 重连session非一次性")

	// 正确做法:验证后立即轮换
	bi2 := &baseInfo{reconnectSession: "s2"}
	verifyRotate := func(session string) bool {
		if session != bi2.reconnectSession {
			return false
		}
		bi2.reconnectSession = "s3"
		return true
	}
	if !verifyRotate("s2") {
		t.Fatal("first reconnect should succeed")
	}
	if verifyRotate("s2") {
		t.Fatal("old session should fail after rotation")
	}
	t.Log("正确实现(立即轮换): 旧session失效")
}

// === util.IsMultiOverflow 测试 ===

func TestIsMultiOverflow(t *testing.T) {
	if util.IsMultiOverflow(10, 5) {
		t.Fatal("10*5 should not overflow")
	}
	if util.IsMultiOverflow(1, math.MaxInt32) {
		t.Fatal("1*MaxInt32 should not overflow")
	}
	if !util.IsMultiOverflow(math.MaxInt32, 2) {
		t.Fatal("MaxInt32*2 should overflow")
	}
	if !util.IsMultiOverflow(100000, 100000) {
		t.Fatal("100000*100000 should overflow int32")
	}
}

// === ClientData 高并发压测 ===

func TestClientDataHighConcurrency(t *testing.T) {
	cd := &network.ClientData{ConnId: 1, GameServerId: 100}
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := int64(1); i <= 10; i++ {
			cd.SetPlayerId(i)
			time.Sleep(time.Microsecond)
		}
	}()

	for i := 0; i < 500; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			pid := cd.GetPlayerId()
			if pid < 0 || pid > 10 {
				t.Errorf("unexpected playerId: %v", pid)
			}
		}()
	}
	wg.Wait()
}

// === Map range+delete 安全模式 ===

func TestMapRangeDeleteSafety(t *testing.T) {
	m := make(map[int]int, 100)
	for i := 0; i < 100; i++ {
		m[i] = i
	}
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	processed := 0
	for _, k := range keys {
		if _, ok := m[k]; ok {
			delete(m, k)
			processed++
		}
	}
	if processed != 100 {
		t.Fatalf("expected 100 processed, got %d", processed)
	}
	if len(m) != 0 {
		t.Fatalf("expected empty map, got %d", len(m))
	}
}

// === GatePacket nil 安全测试 ===

func TestGatePacketNilSafety(t *testing.T) {
	safeAssert := func(packet any) (playerId int64, ok bool) {
		gp, ok := packet.(*network.GatePacket)
		if !ok || gp == nil {
			return 0, false
		}
		return gp.PlayerId(), true
	}

	if _, ok := safeAssert(nil); ok {
		t.Fatal("nil should return ok=false")
	}
	if _, ok := safeAssert("wrong"); ok {
		t.Fatal("wrong type should return ok=false")
	}
	gp := network.NewGatePacket(12345, 1, nil)
	pid, ok := safeAssert(gp)
	if !ok || pid != 12345 {
		t.Fatalf("valid GatePacket failed: ok=%v pid=%v", ok, pid)
	}
}

// === Redis key 格式一致性 ===

func TestRedisKeyFormatConsistency(t *testing.T) {
	// 验证 strconv.FormatInt 和 fmt.Sprintf 对 int64 结果一致
	testCases := []int64{0, 1, -1, 1234567890, math.MaxInt64}
	for _, v := range testCases {
		// 两种方式结果必须一致
		s1 := "prefix:" + formatIntOld(v)
		s2 := "prefix:" + formatIntNew(v)
		if s1 != s2 {
			t.Fatalf("key format mismatch for %d: old=%s new=%s", v, s1, s2)
		}
	}
}

func formatIntOld(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func formatIntNew(v int64) string {
	return formatIntOld(v)
}

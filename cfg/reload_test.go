package cfg

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/encoding/protodelim"
)

// writeTestFile 在临时目录写文件的小工具,写失败直接Fatal
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

// setMd5Snapshot 测试内安全替换md5快照(包私有全局),并注册Cleanup恢复
func setMd5Snapshot(t *testing.T, m map[string]string) {
	t.Helper()
	snapshotMu.Lock()
	orig := md5Snapshot
	md5Snapshot = m
	snapshotMu.Unlock()
	t.Cleanup(func() {
		snapshotMu.Lock()
		md5Snapshot = orig
		snapshotMu.Unlock()
	})
}

// TestLoadMd5File 验证md5清单读取的四种分支:
// 正常读取/文件缺失/JSON损坏/内容为空,后三者必须返回error触发调用方的降级策略
func TestLoadMd5File(t *testing.T) {
	dir := t.TempDir()

	// 正常: 合法JSON返回解析结果(文件名须与导表工具真实产出的json_md5.json一致)
	writeTestFile(t, dir, "json_md5.json", `{"a.json":"md5a","b.json":"md5b"}`)
	md5s, err := loadMd5File(dir)
	if err != nil {
		t.Fatalf("合法json_md5.json应解析成功, got err: %v", err)
	}
	if len(md5s) != 2 || md5s["a.json"] != "md5a" || md5s["b.json"] != "md5b" {
		t.Fatalf("解析结果不符: %v", md5s)
	}

	// 缺失: 不存在的目录必须报错
	if _, err := loadMd5File(filepath.Join(dir, "not_exist")); err == nil {
		t.Fatal("json_md5.json缺失时应返回error")
	}

	// 损坏: 非法JSON必须报错,不能静默返回空map
	writeTestFile(t, dir, "json_md5.json", `{invalid json`)
	if _, err := loadMd5File(dir); err == nil {
		t.Fatal("json_md5.json损坏时应返回error")
	}

	// 空: `{}`视为无效(无法区分清单缺失与空目录),必须报错触发降级
	writeTestFile(t, dir, "json_md5.json", `{}`)
	if _, err := loadMd5File(dir); err == nil {
		t.Fatal("json_md5.json内容为空时应返回error")
	}
}

// TestDiffMd5Snapshot 验证快照diff逻辑:
// 只返回内容变更与新增的文件;已删除的文件不进入变更列表(仅告警,内存旧数据无害)
func TestDiffMd5Snapshot(t *testing.T) {
	setMd5Snapshot(t, map[string]string{
		"unchanged.json": "md5-1",
		"changed.json":   "md5-old",
		"removed.json":   "md5-3",
	})
	changed := diffMd5Snapshot(map[string]string{
		"unchanged.json": "md5-1", // 未变,不应出现
		"changed.json":   "md5-new",
		"added.json":     "md5-4", // 新增,应出现
	})
	if len(changed) != 2 {
		t.Fatalf("应检测到2个变更文件(changed/added), got: %v", changed)
	}
	for _, f := range changed {
		if f != "changed.json" && f != "added.json" {
			t.Fatalf("变更列表含意外文件 %q, got: %v", f, changed)
		}
	}
}

// TestMd5ManifestName pb格式支持(commit 9094d874a)的清单文件名映射:
// 清单名必须与DataFileExt严格配套——pb部署下若仍读json_md5.json,清单缺失会
// 静默降级为全量加载(热更功能失效但无报错,只能靠重启恢复)
func TestMd5ManifestName(t *testing.T) {
	orig := DataFileExt
	t.Cleanup(func() { DataFileExt = orig })

	cases := []struct {
		ext  string
		want string
	}{
		{".json", "json_md5.json"}, // json部署
		{".pb", "pb_md5.json"},     // pb部署
		{"json", "json_md5.json"},  // 无点号前缀的宽容处理
		{"", "json_md5.json"},      // 空值回退默认json,不得产出"_md5.json"
	}
	for _, c := range cases {
		DataFileExt = c.ext
		if got := Md5ManifestName(); got != c.want {
			t.Fatalf("DataFileExt=%q 应返回 %q, got %q", c.ext, c.want, got)
		}
	}
}

// TestLoadMd5File_PbManifest pb格式部署下(loadMd5File与Md5ManifestName的端到端接线):
// DataFileExt=.pb时必须读pb_md5.json;若误读json_md5.json(该目录不存在)会触发全量降级
func TestLoadMd5File_PbManifest(t *testing.T) {
	dir := t.TempDir()
	orig := DataFileExt
	t.Cleanup(func() { DataFileExt = orig })

	// 只有pb清单存在;若接线错误去找json_md5.json,应报错
	writeTestFile(t, dir, "pb_md5.json", `{"a.pb":"md5a"}`)
	DataFileExt = ".pb"
	md5s, err := loadMd5File(dir)
	if err != nil {
		t.Fatalf("pb部署下应成功读取pb_md5.json, got err: %v", err)
	}
	if md5s["a.pb"] != "md5a" {
		t.Fatalf("pb清单解析结果不符: %v", md5s)
	}

	// json清单在该目录不存在,证明两种格式的清单确实分开读取
	DataFileExt = ".json"
	if _, err := loadMd5File(dir); err == nil {
		t.Fatal("json部署下不应读到pb_md5.json,应报文件缺失")
	}
}

// TestReload_NoChangeSkipsLoad 真实cfgdata目录: Load成功→建快照→Reload,
// 无变更时应直接返回nil(不触碰任何表数据),且重复调用幂等
// 这是对生产时序"启动全量加载+热更增量重载"的最小复刻
func TestReload_NoChangeSkipsLoad(t *testing.T) {
	dir := "../cfgdata/"
	DataFileExt = ".json"
	if err := Load(dir, nil); err != nil {
		t.Fatalf("前置全量Load失败: %v", err)
	}
	InitMd5Snapshot(dir)
	for i := 0; i < 2; i++ {
		if err := Reload(dir); err != nil {
			t.Fatalf("无变更的第%d次Reload应返回nil, got err: %v", i+1, err)
		}
	}
}

// TestReload_ChangedFileIsReloaded 回归测试:配置文件变更(json_md5.json随之更新)后,
// Reload必须真正重载该文件并替换内存数据——这是热更功能的核心语义.
// 历史缺陷:c3290caee曾误读仓库遗留的md5.json(导表工具从不更新它),diff恒为空,
// 导致所有热更请求静默跳过加载却返回成功,新配置永远不生效;本用例钉死该行为
func TestReload_ChangedFileIsReloaded(t *testing.T) {
	dir := t.TempDir()
	DataFileExt = ".json"
	// 模拟启动时已加载的旧配置:磁盘是v1,内存是旧数据(空表),快照与磁盘md5一致
	// (ItemCfgs必须显式置为可控旧值:同包其他测试会加载真实cfgdata污染全局状态,不能假设初始为nil)
	writeTestFile(t, dir, "ItemCfg.json", `{"1":{"CfgId":1}}`)
	writeTestFile(t, dir, "json_md5.json", `{"ItemCfg.json":"md5-v1"}`)
	setMd5Snapshot(t, map[string]string{"ItemCfg.json": "md5-v1"})
	// 保存/恢复会被Load写脏的全局状态,保证测试隔离
	origDataDir, origItemCfgs := DataDir, ItemCfgs
	t.Cleanup(func() { DataDir, ItemCfgs = origDataDir, origItemCfgs })
	ItemCfgs = NewDataMap[*pb.ItemCfg]()

	// 未变更时Reload应跳过加载,内存保持旧数据(仍为空表,证明未被替换)
	if err := Reload(dir); err != nil {
		t.Fatalf("无变更的Reload应返回nil, got err: %v", err)
	}
	if len(ItemCfgs.Elems) != 0 {
		t.Fatalf("无变更时不应加载新数据,内存应保持旧空表, got: %v", ItemCfgs.Elems)
	}

	// 模拟策划改表重新导出:数据文件与json_md5.json同步更新
	writeTestFile(t, dir, "ItemCfg.json", `{"1":{"CfgId":1},"2":{"CfgId":2}}`)
	writeTestFile(t, dir, "json_md5.json", `{"ItemCfg.json":"md5-v2"}`)

	if err := Reload(dir); err != nil {
		t.Fatalf("有变更的Reload应成功, got err: %v", err)
	}
	if len(ItemCfgs.Elems) != 2 {
		t.Fatalf("变更后的Reload应重载ItemCfg.json并替换内存数据, want 2 elems, got: %v", ItemCfgs.Elems)
	}
	// 快照应推进到新md5:再Reload一次无变更,数据保持且不报错
	if err := Reload(dir); err != nil {
		t.Fatalf("变更生效后的Reload应无变更直接成功, got err: %v", err)
	}
	if len(ItemCfgs.Elems) != 2 {
		t.Fatalf("无变更的重复Reload不应丢失已加载数据, got: %v", ItemCfgs.Elems)
	}
}

// TestReload_Md5MissingFallsBackToFullLoad md5.json缺失时降级为全量加载:
// 临时目录只放首个表文件(ItemCfg.json)不放md5.json,
// 若走diff路径会直接成功;走全量降级路径则继续逐文件加载,在第二个表文件处报错
func TestReload_Md5MissingFallsBackToFullLoad(t *testing.T) {
	dir := t.TempDir()
	DataFileExt = ".json"
	writeTestFile(t, dir, "ItemCfg.json", `{}`)
	// 保存/恢复会被Load写脏的全局状态,保证测试隔离
	origDataDir, origItemCfgs := DataDir, ItemCfgs
	t.Cleanup(func() { DataDir, ItemCfgs = origDataDir, origItemCfgs })

	err := Reload(dir)
	if err == nil {
		t.Fatal("全量降级路径下临时目录缺少后续表文件,Reload应报错")
	}
	// 报错发生在第二个表文件上,证明确实在做全量逐文件加载而非diff跳过
	if !strings.Contains(err.Error(), "condition_template") {
		t.Fatalf("降级全量加载应推进到第二个表文件condition_template.json, got: %v", err)
	}
}

// writeTestPbFile 写length-delimited格式的pb数据文件(DataMap.LoadPb的读取格式),写失败直接Fatal
func writeTestPbFile(t *testing.T, dir, name string, msgs ...*pb.ItemCfg) {
	t.Helper()
	var buf bytes.Buffer
	for _, m := range msgs {
		if _, err := protodelim.MarshalTo(&buf, m); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0644); err != nil {
		t.Fatal(err)
	}
}

// TestReload_PbChangedFileIsReloaded pb部署(DataFileExt=".pb")的回归测试(commit 9094d874a):
// pb_md5.json的key是解析后的.pb文件名(如ItemCfg.pb),而data_mgr.go注册的表名是.json,
// Reload的filter必须做归一化匹配,否则变更文件全部被filter跳过,
// Reload返回成功但新配置静默不生效,且快照被推进导致后续热更diff恒为空(只能重启恢复)
func TestReload_PbChangedFileIsReloaded(t *testing.T) {
	dir := t.TempDir()
	orig := DataFileExt
	t.Cleanup(func() { DataFileExt = orig })
	DataFileExt = ".pb"

	// 模拟启动时已加载:磁盘是v1(1条数据),内存是旧空表,快照与磁盘md5一致
	writeTestPbFile(t, dir, "ItemCfg.pb", &pb.ItemCfg{CfgId: 1})
	writeTestFile(t, dir, "pb_md5.json", `{"ItemCfg.pb":"md5-v1"}`)
	setMd5Snapshot(t, map[string]string{"ItemCfg.pb": "md5-v1"})
	origDataDir, origItemCfgs := DataDir, ItemCfgs
	t.Cleanup(func() { DataDir, ItemCfgs = origDataDir, origItemCfgs })
	ItemCfgs = NewDataMap[*pb.ItemCfg]()

	// 模拟策划改表重新导出:数据文件与pb_md5.json同步更新为v2(2条数据)
	writeTestPbFile(t, dir, "ItemCfg.pb", &pb.ItemCfg{CfgId: 1}, &pb.ItemCfg{CfgId: 2})
	writeTestFile(t, dir, "pb_md5.json", `{"ItemCfg.pb":"md5-v2"}`)

	if err := Reload(dir); err != nil {
		t.Fatalf("pb部署下有变更的Reload应成功, got err: %v", err)
	}
	if len(ItemCfgs.Elems) != 2 {
		t.Fatalf("pb部署下Reload应重载ItemCfg.pb并替换内存数据, want 2 elems, got: %v", ItemCfgs.Elems)
	}
}

// TestReload_FailedLoadKeepsSnapshot 加载失败不更新快照:
// md5清单标记了变更文件但目录中无该文件时,Reload报错且快照保持旧值,
// 保证下次热更会重试失败文件(快照与实际生效配置严格一致)
func TestReload_FailedLoadKeepsSnapshot(t *testing.T) {
	dir := t.TempDir()
	DataFileExt = ".json"
	writeTestFile(t, dir, "json_md5.json", `{"ItemCfg.json":"md5-new"}`)
	setMd5Snapshot(t, map[string]string{"ItemCfg.json": "md5-old"})
	origDataDir := DataDir
	t.Cleanup(func() { DataDir = origDataDir })

	if err := Reload(dir); err == nil {
		t.Fatal("变更文件不存在时Reload应报错")
	}
	snapshotMu.RLock()
	got := md5Snapshot["ItemCfg.json"]
	snapshotMu.RUnlock()
	if got != "md5-old" {
		t.Fatalf("加载失败时快照不应更新, want md5-old, got %q", got)
	}
}

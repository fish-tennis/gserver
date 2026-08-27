package cfg

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Md5ManifestName 根据配置数据文件格式(DataFileExt)返回导表工具生成的md5清单文件名:
// json格式对应json_md5.json,pb格式对应pb_md5.json
// (见excel/tool/exporter_server.yaml的Md5ExportPath,与ExportFormats一一对应);
// 清单的key与Load实际加载的数据文件名严格一致(大小写敏感),
// 因此热更diff必须用与当前加载格式配套的那份清单,否则key对不上会误判为全量变更
func Md5ManifestName() string {
	ext := strings.TrimPrefix(DataFileExt, ".")
	if ext == "" {
		ext = "json"
	}
	return ext + "_md5.json"
}

var (
	// 上次成功加载的配置文件md5快照: map[文件名]md5
	// 热更时与新导出的md5.json做diff,只重新加载有变更的文件
	// 注意: 不能拿磁盘文件内容与md5.json对比(两者永远一致,排不了重),
	// 必须与"上次实际加载进内存"的快照对比
	md5Snapshot = make(map[string]string)
	snapshotMu  sync.RWMutex
	// 是否处于热更重载流程中
	// 各配置表的AfterLoad钩子(如exchangeAfterLoad)据此区分校验策略:
	// 启动加载时配置问题返回error拒绝启动(fail fast);
	// 热更时仅告警不阻断,避免个别坏配置让整个热更流程中断
	reloading atomic.Bool
)

// IsHotReloading 当前是否处于热更重载流程
func IsHotReloading() bool {
	return reloading.Load()
}

// InitMd5Snapshot 服务器启动时建立md5快照,在启动加载(cfg.Load)成功后调用
// 建立快照后,首次热更即走真正的增量路径,全生命周期行为一致:
// md5清单(按DataFileExt选json_md5.json或pb_md5.json)成为唯一的变更判定依据(与导表工具的产出严格对齐)
// md5清单不可用时快照保持为空,首次热更自动退回全量加载(零风险),因此仅告警不报错
func InitMd5Snapshot(dataDir string) {
	md5s, err := loadMd5File(dataDir)
	if err != nil {
		slog.Warn("InitMd5Snapshot: md5清单不可用,首次热更将执行全量加载", "file", Md5ManifestName(), "err", err)
		return
	}
	snapshotMu.Lock()
	md5Snapshot = md5s
	snapshotMu.Unlock()
	slog.Info("InitMd5Snapshot: md5快照已建立", "files", len(md5s))
}

// Reload 热更配置表入口(与启动时的全量Load相区分):
//   - 读取导表工具生成的md5.json,与上次成功加载的快照diff
//   - 无任何变更: 直接返回,零开销(连Process索引钩子都不重跑)
//   - 有变更: 只重新解析变更文件(json反序列化是耗时大头);
//     但Process钩子仍会全量重跑,因为存在跨表派生数据
//     (如ExchangeIdsByActivity由ActivityCfgs+ExchangeCfgs两张表联合派生),
//     若只重载一张表而跳过另一张表的钩子,派生索引会是脏的;
//     钩子只是重建索引且幂等,全量重跑开销极小
//   - md5.json缺失/损坏/为空: 保守降级为全量加载(等同原热更行为)
//   - 加载失败: 不更新快照直接返回error,下次热更会重试全部变更文件;
//     LoadConfig先解析到临时对象、成功后才替换全局指针,
//     失败时未替换到新数据的表继续沿用旧配置
//   - 快照由启动时的InitMd5Snapshot建立;若启动时md5.json不可用(快照为空),
//     本次热更等价于全量加载,成功后快照建立,之后恢复正常增量
func Reload(dataDir string) error {
	reloading.Store(true)
	defer reloading.Store(false)

	newMd5s, err := loadMd5File(dataDir)
	if err != nil {
		// md5清单不可用就无法diff,保守降级为全量加载
		slog.Warn("Reload: md5清单不可用,降级为全量加载", "file", Md5ManifestName(), "err", err)
		return Load(dataDir, nil)
	}

	changed := diffMd5Snapshot(newMd5s)
	if len(changed) == 0 {
		slog.Info("Reload: 配置无变更,跳过加载")
		return nil
	}
	slog.Info("Reload: 检测到配置变更,只重载变更文件", "changedFiles", changed)

	// filter返回true表示需要加载该文件,未变更的文件跳过解析
	// 注意:pb部署下md5清单key是解析后的文件名(如ItemCfg.pb),而data_mgr.go注册的表名是json名(ItemCfg.json),
	// 必须用ResolveDataFile归一化后再比较;否则changed永远匹配不上,所有表被filter跳过,
	// Reload返回成功但新配置静默不生效,且快照被推进导致后续热更diff恒为空(只能重启恢复)
	if err := Load(dataDir, func(fileName string) bool {
		resolved := ResolveDataFile(fileName)
		for _, f := range changed {
			if f == fileName || f == resolved {
				return true
			}
		}
		return false
	}); err != nil {
		// 失败不更新快照: 下次热更会把这些变更文件重新加载一遍
		return err
	}
	// 全部成功后才更新快照,保证快照与实际生效的配置严格一致
	snapshotMu.Lock()
	md5Snapshot = newMd5s
	snapshotMu.Unlock()
	return nil
}

// loadMd5File 读取导表工具生成的md5清单(具体文件名由Md5ManifestName按DataFileExt决定)
func loadMd5File(dataDir string) (map[string]string, error) {
	fileName := Md5ManifestName()
	path := filepath.ToSlash(filepath.Join(dataDir, fileName))
	fileData, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	md5s := make(map[string]string)
	if err := json.Unmarshal(fileData, &md5s); err != nil {
		return nil, fmt.Errorf("%s解析失败: %w", fileName, err)
	}
	if len(md5s) == 0 {
		return nil, fmt.Errorf("%s内容为空", fileName)
	}
	return md5s, nil
}

// diffMd5Snapshot 对比新md5清单与内存快照,返回有变更(含新增)的文件列表
func diffMd5Snapshot(newMd5s map[string]string) []string {
	snapshotMu.RLock()
	defer snapshotMu.RUnlock()
	var changed []string
	for file, md5 := range newMd5s {
		if old, ok := md5Snapshot[file]; !ok || old != md5 {
			changed = append(changed, file)
		}
	}
	// 快照中有但新清单中没有的文件: 说明导表配置删掉了该表,
	// data_mgr.go会随之重新生成并随新二进制发布,此处内存旧数据无害,仅告警
	for file := range md5Snapshot {
		if _, ok := newMd5s[file]; !ok {
			slog.Warn("Reload: config file removed from md5.json, keep stale data in memory", "file", file)
		}
	}
	return changed
}

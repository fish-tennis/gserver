package cfg

import (
	"fmt"
	"log/slog"

	"github.com/fish-tennis/gserver/pb"
)

var (
	// 按ExchangeCategory分类的兑换配置索引,新增ExchangeCategory时无需再改此处即可按分类访问
	ExchangesByCategory map[int32]*DataMap[*pb.ExchangeCfg]
)

func init() {
	register.ExchangeCfgsProcess = exchangeAfterLoad
}

// IsUnsafeFreeExchange 判断是否为"免费且不限次数且非充值项"的礼包配置
// 这种组合是必然的漏洞配置: 满足条件的玩家可以无限次无代价领取奖励,不存在合法业务场景
//   - 条件模板(ConditionTemplates)救不了: 条件是状态检查(如活跃度>=X),满足后可反复通过,不是一次性闸门
//   - 充值项(RechargeCfgId>0)不算: OnExchangeReq入口已拦截,只能由充值回调按服务器写死的份数触发
// 供配置加载告警与运行期兑换拦截两处复用,保证判定口径一致
func IsUnsafeFreeExchange(e *pb.ExchangeCfg) bool {
	return e.GetCountLimit() <= 0 && len(e.GetConsumes()) == 0 && e.GetRechargeCfgId() <= 0
}

func exchangeAfterLoad(mgr *DataMap[*pb.ExchangeCfg]) error {
	// 统一把所有兑换配置的条件模板转换为运行期条件,仅在此处转换一次,供下面的子集/列表复用
	mgr.Range(func(e *pb.ExchangeCfg) bool {
		e.Conditions = ConvertConditionCfgs(e.ConditionTemplates)
		return true
	})
	// 按Category全量分组,任何ExchangeCategory自动进入索引,取代以前每个分类一个硬编码变量的写法
	ExchangesByCategory = mgr.CreateIndexInt32(func(e *pb.ExchangeCfg) int32 {
		return e.GetCategory()
	})
	// 配置合法性检查: 扫描"免费且不限次数且非充值项"的漏洞配置
	// 按加载场景区分策略(通过IsHotReloading判断,见reload.go):
	//   - 启动加载: 返回error拒绝启动,把配置错误暴露在最早期(fail fast)
	//   - 热更重载: 仅Error告警不阻断,避免个别坏配置中断整个热更流程(其余新配置全部不生效);
	//     此时真正的拦截由运行期Exchange入口的IsUnsafeFreeExchange检查兜底
	var illegal []int32
	mgr.Range(func(e *pb.ExchangeCfg) bool {
		if IsUnsafeFreeExchange(e) {
			illegal = append(illegal, e.GetCfgId())
		}
		return true
	})
	if len(illegal) > 0 {
		if IsHotReloading() {
			slog.Error("exchange.xlsx has free unlimited non-recharge exchange cfgs (exploitable, will be blocked at runtime), CountLimit should be >=1 or Consumes should be set",
				"exchangeCfgIds", illegal)
			return nil
		}
		return fmt.Errorf("exchange.xlsx has free unlimited non-recharge exchange cfgs (exploitable), CountLimit should be >=1 or Consumes should be set: %v", illegal)
	}
	return nil
}

// GetExchangesByCategory 按兑换分类获取配置子集
// 分类不存在时返回空DataMap而非nil,调用方可直接Range无需判空
func GetExchangesByCategory(category int32) *DataMap[*pb.ExchangeCfg] {
	if subMap, ok := ExchangesByCategory[category]; ok {
		return subMap
	}
	return NewDataMap[*pb.ExchangeCfg]()
}

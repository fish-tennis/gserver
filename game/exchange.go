package game

import (
	"errors"
	"math"

	"log/slog"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
)

const (
	// 组件名
	ComponentNameExchange = "Exchange"
)

// 利用go的init进行组件的自动注册
func init() {
	_playerComponentRegister.Register(ComponentNameExchange, 0, func(player *Player, _ any) gentity.Component {
		return &Exchange{
			BasePlayerComponent: BasePlayerComponent{
				player: player,
				name:   ComponentNameExchange,
			},
			Records: gentity.NewMapData[int32, *pb.ExchangeRecord](),
		}
	})
}

// 兑换模块
type Exchange struct {
	BasePlayerComponent
	// 兑换记录
	Records *gentity.MapData[int32, *pb.ExchangeRecord] `db:""`
}

func (p *Player) GetExchange() *Exchange {
	return p.GetComponentByName(ComponentNameExchange).(*Exchange)
}

func (e *Exchange) OnDataLoad() {
}

// OnEvent 处理跨日事件:清除玩家已有的非活动每日刷新礼包的兑换记录,使玩家次日可再次兑换
// 活动礼包不在此处处理,由活动模块(ActivityDefault)按活动自身的刷新类型进行重置,
// 避免活动礼包(如以活动开启周期为刷新依据的)在活动期间被误清
func (e *Exchange) OnEvent(event interface{}) {
	switch event.(type) {
	case *EventDateChange:
		// 只遍历玩家身上已有的兑换记录,避免每天全量扫描配置表;
		// 没有记录的礼包本身就没有兑换次数,无需处理
		e.Records.Range(func(exchangeCfgId int32, _ *pb.ExchangeRecord) bool {
			// 属于活动的礼包跳过,交由活动模块按活动刷新周期统一重置
			if cfg.GetActivityIdByExchangeId(exchangeCfgId) > 0 {
				return true
			}
			exchangeCfg := cfg.ExchangeCfgs.GetCfg(exchangeCfgId)
			if exchangeCfg != nil && exchangeCfg.GetRefreshType() == int32(pb.RefreshType_RefreshType_Day) {
				// Go遍历map时删除当前或未遍历到的key是安全的
				e.RemoveRecord(exchangeCfgId)
			}
			return true
		})
	}
}

func (e *Exchange) SyncDataToClient() {
	e.GetPlayer().Send(&pb.ExchangeSync{
		Records: e.Records.Data,
	})
}

// 获取已兑换次数
func (e *Exchange) GetCount(exchangeCfgId int32) int32 {
	return e.Records.Data[exchangeCfgId].GetCount()
}

func (e *Exchange) addExchangeCount(exchangeCfgId, exchangeCount int32) {
	v, ok := e.Records.Get(exchangeCfgId)
	if !ok {
		v = &pb.ExchangeRecord{
			CfgId: exchangeCfgId,
		}
	}
	// 用 int64 运算防止累加溢出,结果限制在 int32 范围内
	newCount := int64(v.Count) + int64(exchangeCount)
	if newCount > math.MaxInt32 {
		newCount = math.MaxInt32
	}
	v.Count = int32(newCount)
	v.Timestamp = int32(e.GetPlayer().GetTimerEntries().Now().Unix())
	e.Records.Set(exchangeCfgId, v)
	e.GetPlayer().Send(&pb.ExchangeUpdate{
		Records: []*pb.ExchangeRecord{v},
	})
}

func (e *Exchange) RemoveRecord(exchangeCfgId int32) *pb.ExchangeRecord {
	if v, ok := e.Records.Get(exchangeCfgId); ok {
		e.Records.Delete(exchangeCfgId)
		e.GetPlayer().Send(&pb.ExchangeRemove{
			CfgIds: []int32{exchangeCfgId},
		})
		return v
	}
	return nil
}

func (e *Exchange) GetRecordsByIds(exchangeCfgId ...int32) (records []*pb.ExchangeRecord) {
	for _, id := range exchangeCfgId {
		if v, ok := e.Records.Get(id); ok {
			records = append(records, v)
		}
	}
	return
}

// 兑换物品
//
//	商店也可以看作是一种兑换功能
func (e *Exchange) Exchange(exchangeCfgId, exchangeCount int32) error {
	if exchangeCount <= 0 {
		return errors.New("exchangeCount <= 0")
	}
	exchangeCfg := cfg.ExchangeCfgs.GetCfg(exchangeCfgId)
	if exchangeCfg == nil {
		slog.Debug("Exchange exchangeCfg nil", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId)
		return errors.New("exchangeCfg nil")
	}
	// 运行期安全兜底: 拦截"免费且不限次数且非充值项"的漏洞配置
	// 这种配置下CountLimit检查形同虚设,玩家可无限次/大Count白嫖奖励(如单次Count=1000直接放大1000倍);
	// 配置加载时已对这类配置打Error告警,但热更场景下不能阻断加载,故此处必须再兜底拦截
	if cfg.IsUnsafeFreeExchange(exchangeCfg) {
		slog.Error("Exchange unsafeExchangeCfg",
			"pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId, "exchangeCount", exchangeCount)
		return errors.New("unsafeExchangeCfg")
	}
	curExchangeCount := e.GetCount(exchangeCfgId)
	if exchangeCfg.CountLimit > 0 && int64(curExchangeCount)+int64(exchangeCount) > int64(exchangeCfg.CountLimit) {
		slog.Debug("Exchange CountLimit", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId, "exchangeCount", exchangeCount)
		return errors.New("exchangeCountLimit")
	}
	// 检查兑换条件
	var obj any
	activityId := cfg.GetActivityIdByExchangeId(exchangeCfgId)
	// NOTE:活动礼包比较特殊,需要获取到活动对象,这样CheckConditions才能正确检查活动条件
	if activityId > 0 {
		obj = e.GetPlayer().GetActivities().GetActivity(activityId)
	}
	if obj == nil {
		obj = e.GetPlayer()
	}
	if !CheckConditions(obj, exchangeCfg.Conditions) {
		slog.Debug("conditions err", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId)
		return errors.New("conditions err")
	}
	// 如果配置了兑换消耗物品,就是购买礼包,如果不配置,就是免费礼包
	// 注意:必须深拷贝后再修改,不能污染全局cfg配置数据
	var totalConsumes []*pb.ItemNum
	if exchangeCount > 1 {
		totalConsumes = make([]*pb.ItemNum, len(exchangeCfg.Consumes))
		for i, consume := range exchangeCfg.Consumes {
			if util.IsMultiOverflow(consume.Num, exchangeCount) {
				slog.Debug("Exchange ConsumeItems overflow", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId, "exchangeCount", exchangeCount)
				return errors.New("ConsumeItemsOverflow")
			}
			totalConsumes[i] = &pb.ItemNum{
				CfgId: consume.CfgId,
				Num:   consume.Num * exchangeCount,
			}
		}
	} else {
		totalConsumes = exchangeCfg.Consumes
	}
	var totalRewards []*pb.AddElemArg
	if exchangeCount > 1 {
		totalRewards = make([]*pb.AddElemArg, len(exchangeCfg.Rewards))
		for i, reward := range exchangeCfg.Rewards {
			if util.IsMultiOverflow(reward.Num, exchangeCount) {
				slog.Debug("Exchange rewards overflow", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId, "exchangeCount", exchangeCount)
				return errors.New("RewardsItemsOverflow")
			}
			totalRewards[i] = &pb.AddElemArg{
				CfgId:      reward.CfgId,
				Num:        reward.Num * exchangeCount,
				TimeType:   reward.TimeType,
				Timeout:    reward.Timeout,
				Source:     reward.Source,
				Properties: reward.Properties,
			}
		}
	} else {
		totalRewards = exchangeCfg.Rewards
	}
	if !e.GetPlayer().GetBags().IsEnoughByItemNums(totalConsumes) {
		slog.Debug("Exchange ConsumeItems notEnough", "pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId)
		return errors.New("ConsumeItemsNotEnough")
	}
	// 当前顺序为: 记录次数→扣除消耗→发放奖励,任一步失败无回滚,设计如此,非bug,游戏防刷需要
	e.addExchangeCount(exchangeCfgId, exchangeCount)          // 先记录兑换次数
	e.GetPlayer().GetBags().DelItemsByItemNums(totalConsumes) // 消耗
	e.GetPlayer().GetBags().AddItems(totalRewards)            // 最后给奖励
	return nil
}

// 响应客户端的兑换请求(购买物品,兑换礼包,领取奖励等)
func (e *Exchange) OnExchangeReq(req *pb.ExchangeReq) (*pb.ExchangeRes, error) {
	// 拦截: RechargeCfgId > 0的兑换项是充值项,只能由充值回调触发,玩家无法手动领取
	for _, idCount := range req.GetIdCounts() {
		exchangeCfg := cfg.ExchangeCfgs.GetCfg(idCount.GetId())
		if exchangeCfg != nil && exchangeCfg.RechargeCfgId > 0 {
			return nil, errors.New("该物品需要通过充值获取")
		}
	}
	res := &pb.ExchangeRes{}
	for _, idCount := range req.GetIdCounts() {
		err := e.Exchange(idCount.GetId(), idCount.GetCount())
		if err != nil {
			continue
		}
		res.IdCounts = append(res.IdCounts, idCount)
		res.Records = append(res.Records, e.GetRecordsByIds(idCount.GetId())...)
	}
	return res, nil
}

// ExchangeForRecharge 充值回调专用兑换入口
//
//	与Exchange()的区别: 不拦截RechargeCfgId > 0的配置(因为是充值回调触发,而非玩家手动调用)
//	内部逻辑与Exchange()完全一致: CountLimit检查→条件检查→记录次数→发放奖励
//	充值项的次数限制、刷新规则、首充条件等全部复用ExchangeCfg现有机制
func (e *Exchange) ExchangeForRecharge(exchangeCfgId, exchangeCount int32) (rewards []*pb.AddElemArg, err error) {
	if exchangeCount <= 0 {
		return nil, errors.New("exchangeCount <= 0")
	}
	exchangeCfg := cfg.ExchangeCfgs.GetCfg(exchangeCfgId)
	if exchangeCfg == nil {
		slog.Debug("ExchangeForRecharge exchangeCfg nil",
			"pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId)
		return nil, errors.New("exchangeCfg nil")
	}
	// 充值项必须是RechargeCfgId > 0的配置
	if exchangeCfg.RechargeCfgId <= 0 {
		return nil, errors.New("not a recharge exchange cfg")
	}
	curExchangeCount := e.GetCount(exchangeCfgId)
	if exchangeCfg.CountLimit > 0 && int64(curExchangeCount)+int64(exchangeCount) > int64(exchangeCfg.CountLimit) {
		slog.Debug("ExchangeForRecharge CountLimit",
			"pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId,
			"exchangeCount", exchangeCount)
		return nil, errors.New("exchangeCountLimit")
	}
	// 检查兑换条件(如首充条件等)
	var obj any
	activityId := cfg.GetActivityIdByExchangeId(exchangeCfgId)
	if activityId > 0 {
		obj = e.GetPlayer().GetActivities().GetActivity(activityId)
	}
	if obj == nil {
		obj = e.GetPlayer()
	}
	if !CheckConditions(obj, exchangeCfg.Conditions) {
		slog.Debug("ExchangeForRecharge conditions err",
			"pid", e.GetPlayer().GetId(), "exchangeCfgId", exchangeCfgId)
		return nil, errors.New("conditions err")
	}
	var totalRewards []*pb.AddElemArg
	if exchangeCount > 1 {
		totalRewards = make([]*pb.AddElemArg, len(exchangeCfg.Rewards))
		for i, reward := range exchangeCfg.Rewards {
			if util.IsMultiOverflow(reward.Num, exchangeCount) {
				return nil, errors.New("RewardsItemsOverflow")
			}
			totalRewards[i] = &pb.AddElemArg{
				CfgId:      reward.CfgId,
				Num:        reward.Num * exchangeCount,
				TimeType:   reward.TimeType,
				Timeout:    reward.Timeout,
				Source:     reward.Source,
				Properties: reward.Properties,
			}
		}
	} else {
		totalRewards = exchangeCfg.Rewards
	}
	// 记录次数→发放奖励(顺序与Exchange一致)
	e.addExchangeCount(exchangeCfgId, exchangeCount)
	e.GetPlayer().GetBags().AddItems(totalRewards)
	return totalRewards, nil
}

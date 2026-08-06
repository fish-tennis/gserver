package game

import (
	"errors"
	"math"

	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gserver/cfg"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"log/slog"
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

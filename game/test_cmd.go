package game

import (
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"strings"
)

// 直接写在实体上的消息回调
func (p *Player) OnTestCmd(req *pb.TestCmd) {
	slog.Info("OnTestCmd", "cmd", req.Cmd)
	// NOTE: 实际项目中,这里要检查一下是否是测试环境
	cmd := gnet.PacketCommand(network.GetCommandByProto(req))
	cmdStrs := strings.Split(req.GetCmd(), " ")
	if len(cmdStrs) == 0 {
		p.SendErrorRes(cmd, "empty cmd")
		return
	}
	cmdKey := strings.ToLower(cmdStrs[0])
	cmdArgs := cmdStrs[1:]
	switch cmdKey {
	case strings.ToLower("AddExp"):
		// 加经验值
		if len(cmdArgs) != 1 {
			p.SendErrorRes(cmd, "AddExp cmdArgs error")
			return
		}
		value := int32(util.Atoi(cmdArgs[0]))
		if value < 1 {
			p.SendErrorRes(cmd, "AddExp value error")
			return
		}
		p.GetBaseInfo().IncExp(value)

	case strings.ToLower("AddItem"):
		// 加物品
		if len(cmdArgs) < 1 {
			p.SendErrorRes(cmd, "AddItem cmdArgs error")
			return
		}
		itemCfgId := int32(util.Atoi(cmdArgs[0]))
		itemCfg := cfg.GetItemCfgMgr().GetItemCfg(itemCfgId)
		if itemCfg == nil {
			p.SendErrorRes(cmd, "AddItem itemCfgId error")
			return
		}
		itemNum := int32(1)
		if len(cmdArgs) >= 2 {
			itemNum = int32(util.Atoi(cmdArgs[1]))
		}
		if itemNum < 1 {
			p.SendErrorRes(cmd, "AddItem itemNum error")
			return
		}
		p.GetBags().AddItem(&pb.AddItemArg{
			CfgId: itemCfgId,
			Num:   itemNum,
		})

	case strings.ToLower("FinishQuest"), strings.ToLower("FinishQuests"):
		if len(cmdArgs) < 1 {
			p.SendErrorRes(cmd, "FinishQuest cmdArgs error")
			return
		}
		// 完成所有任务
		if strings.ToLower(cmdArgs[0]) == "all" {
			for cfgId, _ := range p.GetQuest().Quests.Data {
				p.GetQuest().OnFinishQuestReq(&pb.FinishQuestReq{
					QuestCfgId: cfgId,
				})
			}
		} else {
			// 完成某一个任务
			cfgId := int32(util.Atoi(cmdArgs[0]))
			p.GetQuest().OnFinishQuestReq(&pb.FinishQuestReq{
				QuestCfgId: cfgId,
			})
		}

	case strings.ToLower("Fight"):
		// 模拟一个战斗事件
		evt := &pb.EventFight{
			PlayerId: p.GetId(),
		}
		if len(cmdArgs) >= 1 && cmdArgs[0] == "true" {
			evt.IsPvp = true
		}
		if len(cmdArgs) >= 2 && cmdArgs[1] == "true" {
			evt.IsWin = true
		}
		p.FireConditionEvent(evt)

	case strings.ToLower("SignIn"):
		evt := &pb.EventPlayerPropertyInc{
			PlayerId:      p.GetId(),
			PropertyName:  "SignIn", // 签到事件
			PropertyValue: 1,
		}
		p.FireEvent(evt)

	case strings.ToLower("Pay"):
		if len(cmdArgs) < 1 {
			p.SendErrorRes(cmd, "Pay cmdArgs error")
			return
		}
		payValue := int32(util.Atoi(cmdArgs[0]))
		p.GetBaseInfo().Data.TotalPay += payValue
		evt := &pb.EventPlayerPropertyInc{
			PlayerId:      p.GetId(),
			PropertyName:  "TotalPay",
			PropertyValue: payValue,
		}
		p.FireEvent(evt)

	case strings.ToLower("PlayerPropertyInc"):
		if len(cmdArgs) < 2 {
			p.SendErrorRes(cmd, "PlayerPropertyInc cmdArgs error")
			return
		}
		evt := &pb.EventPlayerPropertyInc{
			PlayerId:      p.GetId(),
			PropertyName:  cmdArgs[0],
			PropertyValue: int32(util.Atoi(cmdArgs[1])),
		}
		p.FireEvent(evt)

	case strings.ToLower("AddActivity"):
		if len(cmdArgs) < 1 {
			p.SendErrorRes(cmd, "AddActivity cmdArgs error")
			return
		}
		arg := cmdArgs[0]
		if arg == "all" {
			p.GetActivities().AddAllActivities(p.GetTimerEntries().Now())
		} else {
			activityId := int32(util.Atoi(arg))
			activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
			p.GetActivities().AddNewActivity(activityCfg, p.GetTimerEntries().Now())
		}

	case strings.ToLower("ActivityReceiveReward"):
		if len(cmdArgs) < 2 {
			p.SendErrorRes(cmd, "ActivityReceiveReward cmdArgs error")
			return
		}
		activityId := int32(util.Atoi(cmdArgs[0]))
		cfgId := int32(util.Atoi(cmdArgs[1]))
		activity := p.GetActivities().GetActivity(activityId)
		if activity == nil {
			return
		}
		if activityDefault, ok := activity.(*ActivityDefault); ok {
			activityDefault.ReceiveReward(cfgId)
		}

	case strings.ToLower("ActivityExchange"):
		if len(cmdArgs) < 2 {
			p.SendErrorRes(cmd, "ActivityExchange cmdArgs error")
			return
		}
		activityId := int32(util.Atoi(cmdArgs[0]))
		cfgId := int32(util.Atoi(cmdArgs[1]))
		activity := p.GetActivities().GetActivity(activityId)
		if activity == nil {
			return
		}
		if activityDefault, ok := activity.(*ActivityDefault); ok {
			activityDefault.Exchange(cfgId)
		}

	case strings.ToLower("GuildRouteError"):
		// 模拟一个rpc错误,向一个不存在的公会发送rpc消息
		reply := new(pb.GuildJoinRes)
		err := p.GetGuild().RouteRpcToTargetGuild(123456789,
			&pb.GuildJoinReq{Id: 123456789}, reply)
		if err != nil {
			slog.Info("GuildRouteError", "err", err.Error())
			return
		}
		slog.Debug("GuildRouteError reply", "reply", reply)

	default:
		p.SendErrorRes(cmd, "unsupport test cmd")
	}
}

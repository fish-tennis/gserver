package game

import (
	"fmt"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/network"
	"github.com/fish-tennis/gserver/pb"
	"log/slog"
	"reflect"
	"strconv"
	"strings"
)

// 客户端输入的测试命令
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
		addItemArg := &pb.AddElemArg{
			CfgId: itemCfgId,
			Num:   1,
		}
		if len(cmdArgs) > 1 {
			addItemArg.Num = int32(util.Atoi(cmdArgs[1]))
		}
		// 参数3: 限时秒数
		if len(cmdArgs) > 2 {
			addItemArg.TimeType = int32(pb.TimeType_TimeType_Timestamp)
			addItemArg.Timeout = int32(util.Atoi(cmdArgs[2]))
		}
		if addItemArg.Num < 1 {
			p.SendErrorRes(cmd, "AddItem itemNum error")
			return
		}
		p.GetBags().AddItems([]*pb.AddElemArg{addItemArg})

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

	case strings.ToLower("PlayerProperty"):
		if len(cmdArgs) < 2 {
			p.SendErrorRes(cmd, "PlayerProperty cmdArgs error")
			return
		}
		// 模拟玩家属性更新事件
		evt := &pb.EventPlayerProperty{
			PlayerId: p.GetId(),
			Property: cmdArgs[0],
			Delta:    int32(util.Atoi(cmdArgs[1])),
			Current:  p.GetPropertyInt32(cmdArgs[0]),
		}
		p.FireEvent(evt)

	case strings.ToLower("AddActivity"):
		if len(cmdArgs) < 1 {
			p.SendErrorRes(cmd, "AddActivity cmdArgs error")
			return
		}
		arg := cmdArgs[0]
		if arg == "all" {
			p.GetActivities().AddAllActivitiesCanJoin(p.GetTimerEntries().Now())
		} else {
			activityId := int32(util.Atoi(arg))
			activityCfg := cfg.GetActivityCfgMgr().GetActivityCfg(activityId)
			// 如果已有该活动,则重置
			p.GetActivities().AddNewActivity(activityCfg, p.GetTimerEntries().Now())
		}

	case strings.ToLower("ActivityExchange"):
		if len(cmdArgs) < 2 {
			p.SendErrorRes(cmd, "ActivityExchange cmdArgs error")
			return
		}
		activityId := int32(util.Atoi(cmdArgs[0]))
		cfgId := int32(util.Atoi(cmdArgs[1]))
		p.GetActivities().OnActivityExchangeReq(&pb.ActivityExchangeReq{
			ActivityId:    activityId,
			ExchangeCfgId: cfgId,
			ExchangeCount: 1,
		})

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
		// 通用的客户端请求消息 proto消息名 字段名1 字段值1 字段名2 字段值2
		// 如 ActivityExchangeReq ActivityId 1 ExchangeCfgId 10001 ExchangeCount 1
		messageName := cmdStrs[0]
		newMsg := network.NewMessageByName(messageName)
		if newMsg != nil {
			msgVal := reflect.ValueOf(newMsg).Elem()
			for i := 0; i < len(cmdArgs)/2; i++ {
				fieldName := cmdArgs[i*2]
				fieldValue := cmdArgs[i*2+1]
				fieldVal := msgVal.FieldByName(fieldName)
				if !fieldVal.IsValid() {
					slog.Error("OnTestCmd fieldNameError", "messageName", messageName, "fieldName", fieldName)
					continue
				}
				switch fieldVal.Kind() {
				case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
					fieldVal.SetInt(int64(util.Atoi(fieldValue)))
				case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
					fieldVal.SetUint(util.Atou(fieldValue))
				case reflect.Float32:
					f, _ := strconv.ParseFloat(fieldValue, 32)
					fieldVal.SetFloat(f)
				case reflect.Float64:
					f, _ := strconv.ParseFloat(fieldValue, 64)
					fieldVal.SetFloat(f)
				case reflect.Bool:
					fieldVal.SetBool(strings.ToLower(fieldValue) == "true" || fieldValue == "1")
				case reflect.String:
					fieldVal.SetString(fieldValue)
				default:
					slog.Error("OnTestCmd fieldTypeError", "messageName", messageName, "fieldName", fieldName, "fieldType", fieldVal.Kind())
				}
			}
			slog.Info("mockMessage", "messageName", messageName, "newMsg", newMsg)
			p.PushMessage(network.NewPacket(newMsg))
			return
		}
		p.SendErrorRes(cmd, fmt.Sprintf("unsupported test cmd:%v", cmdKey))
	}
}

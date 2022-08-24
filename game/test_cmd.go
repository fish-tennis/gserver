package game

import (
	"strings"

	"github.com/fish-tennis/gentity/util"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/cfg"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
)

// 客户端字符串形式的测试命令,仅用于测试环境!
func onTestCmd(player *gameplayer.Player, packet *ProtoPacket) {
	logger.Debug("onTestCmd %v", packet.Message())
	req := packet.Message().(*pb.TestCmd)
	cmdStrs := strings.Split(req.GetCmd(), " ")
	if len(cmdStrs) == 0 {
		player.SendErrorRes(packet.Command(), "empty cmd")
		return
	}
	cmdKey := strings.ToLower(cmdStrs[0])
	cmdArgs := cmdStrs[1:]
	switch cmdKey {
	case "addexp":
		// 加经验值
		if len(cmdArgs) != 1 {
			player.SendErrorRes(packet.Command(), "addexp cmdArgs error")
			return
		}
		value := int32(util.Atoi(cmdArgs[0]))
		if value < 1 {
			player.SendErrorRes(packet.Command(), "addexp value error")
			return
		}
		player.GetBaseInfo().IncExp(value)

	case "additem":
		// 加物品
		if len(cmdArgs) < 1 {
			player.SendErrorRes(packet.Command(), "additem cmdArgs error")
			return
		}
		itemCfgId := int32(util.Atoi(cmdArgs[0]))
		itemCfg := cfg.GetItemCfgMgr().GetItemCfg(itemCfgId)
		if itemCfg == nil {
			player.SendErrorRes(packet.Command(), "additem itemCfgId error")
			return
		}
		itemNum := int32(1)
		if len(cmdArgs) >= 2 {
			itemNum = int32(util.Atoi(cmdArgs[1]))
		}
		if itemNum < 1 {
			player.SendErrorRes(packet.Command(), "additem itemNum error")
			return
		}
		player.GetBag().AddItem(itemCfgId, itemNum)

	case "finishquest", "finishquests":
		if len(cmdArgs) < 1 {
			player.SendErrorRes(packet.Command(), "finishquest cmdArgs error")
			return
		}
		// 完成所有任务
		if strings.ToLower(cmdArgs[0]) == "all" {
			for cfgId, _ := range player.GetQuest().Quests.Quests {
				player.GetQuest().OnFinishQuestReq(PacketCommand(pb.CmdQuest_Cmd_FinishQuestReq), &pb.FinishQuestReq{
					QuestCfgId: cfgId,
				})
			}
		} else {
			// 完成某一个任务
			cfgId := int32(util.Atoi(cmdArgs[0]))
			player.GetQuest().OnFinishQuestReq(PacketCommand(pb.CmdQuest_Cmd_FinishQuestReq), &pb.FinishQuestReq{
				QuestCfgId: cfgId,
			})
		}

	case "fight":
		// 模拟一个战斗事件
		evt := &pb.EventFight{
			PlayerId: player.GetId(),
		}
		if len(cmdArgs) >= 1 && cmdArgs[0] == "true" {
			evt.IsPvp = true
		}
		if len(cmdArgs) >= 2 && cmdArgs[1] == "true" {
			evt.IsWin = true
		}
		player.FireConditionEvent(evt)

	default:
		player.SendErrorRes(packet.Command(), "unsupport test cmd")
	}
}

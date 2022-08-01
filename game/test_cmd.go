package game

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"strings"
)

// 客户端字符串形式的测试命令,仅用于测试环境!
func onTestCmd(connection Connection, packet *ProtoPacket) {
	logger.Debug("onTestCmd %v", packet.Message())
	if connection.GetTag() == nil {
		return
	}
	playerId,ok := connection.GetTag().(int64)
	if !ok {
		return
	}
	player := gameplayer.GetPlayerMgr().GetPlayer(playerId)
	if player == nil {
		return
	}
	req := packet.Message().(*pb.TestCmd)
	cmdStrs := strings.Split(req.GetCmd(), " ")
	if len(cmdStrs) == 0 {
		connection.Send(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
			Command: int32(packet.Command()),
			ResultStr: "empty cmd",
		})
		return
	}
	cmdKey := strings.ToLower(cmdStrs[0])
	cmdArgs := cmdStrs[1:]
	switch cmdKey {
	case "addexp":
		// 加经验值
		if len(cmdArgs) != 1 {
			connection.Send(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
				Command: int32(packet.Command()),
				ResultStr: "addexp cmdArgs error",
			})
			return
		}
		value := int32(util.Atoi(cmdArgs[0]))
		if value < 1 {
			connection.Send(PacketCommand(pb.CmdInner_Cmd_ErrorRes), &pb.ErrorRes{
				Command: int32(packet.Command()),
				ResultStr: "addexp cmdArgs error",
			})
			return
		}
		player.GetBaseInfo().IncExp(value)

	case "finishquest","finishquests":
		// 完成任务
		player.GetQuest().FinishQuests()

	case "fight":
		// 模拟一个战斗事件
		evt := &pb.EventFight{
		PlayerId: playerId,
		}
		if len(cmdArgs) >= 1 && cmdArgs[0] == "true" {
			evt.IsPvp = true
		}
		if len(cmdArgs) >= 2 && cmdArgs[1] == "true" {
			evt.IsWin = true
		}
		player.FireConditionEvent(evt)
	}
}
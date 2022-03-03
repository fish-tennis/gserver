package social

import (
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func RoutePlayerPacket(playerId int64, serverId int32, cmd PacketCommand, message proto.Message) {
	player := gameplayer.GetPlayerMgr().GetPlayer(playerId)
	if player != nil {
		player.Send(cmd, message)
		return
	}
	any,err := anypb.New(message)
	if err != nil {
		logger.Error("RoutePlayerPacket %v err:%v", playerId, err)
		return
	}
	internal.GetServerList().SendToServer(serverId, PacketCommand(pb.CmdRoute_Cmd_RoutePlayerMessage), &pb.RoutePlayerMessage{
		ToPlayerId: playerId,
		PacketCommand: int32(cmd),
		PacketData: any,
	})
}

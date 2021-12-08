package game

import (
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
)

func player_component_handler_auto_register(handler *ClientConnectionHandler) {
	//handler.Register(Cmd(pb.CmdMoney_Cmd_CoinReq), func(connection gnet.Connection, packet *gnet.ProtoPacket) {
	//	if connection.GetTag() != nil {
	//		// 在线玩家的消息,自动路由到对应的玩家组件上
	//		player := gameServer.GetPlayer(connection.GetTag().(int64))
	//		if player != nil {
	//			component := player.GetComponent("money")
	//			if component != nil {
	//				OnCoinReqEx(player, component.(*Money), packet.Message().(*pb.CoinReq))// 如果有需要保存的数据修改了,即时保存数据库
	//				player.Save()
	//				return
	//			}
	//		}
	//	}
	//}, func() proto.Message {
	//	return new(pb.CoinReq)
	//})
	handler.RegisterProtoCodeGen("money", Cmd(pb.CmdMoney_Cmd_CoinReq), func() proto.Message {return new(pb.CoinReq)}, func(c Component, m proto.Message) {
		OnCoinReq(c.(*Money), m.(*pb.CoinReq))
	})
	//handler.RegisterProtoCodeGen("{protoName}", Cmd(pb.Cmd{ProtoName}_Cmd_{MessageName}), func() proto.Message {return new(pb.{MessageName})}, func(c Component, m proto.Message) {
	//	On{MessageName}(c.(*{ProtoName}), m.(*pb.{MessageName}))
	//})
}

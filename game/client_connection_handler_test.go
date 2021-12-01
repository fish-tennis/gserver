package game

import (
	"github.com/fish-tennis/gserver/pb"
	"testing"
)

func TestAutoRegister(t *testing.T) {
	connectionHandler := NewClientConnectionHandler(nil)
	//money := &Money{}
	//moneyTyp := reflect.TypeOf(money)
	//method,_ := moneyTyp.MethodByName("OnCoinReq")
	//connectionHandler.RegisterMethod(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinReq),"money", method)

	println(pb.CmdMoney_Cmd_CoinReq.Descriptor().FullName())

	connectionHandler.autoRegisterPlayerComponentProto()

	//packet := gnet.NewProtoPacket(gnet.PacketCommand(pb.CmdMoney_Cmd_CoinReq),&pb.CoinReq{
	//	Coin: 3,
	//})
	//connectionHandler.OnRecvPacket(nil, packet)
}

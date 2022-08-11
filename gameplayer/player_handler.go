package gameplayer

import (
	"fmt"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
	"github.com/fish-tennis/gserver/logger"
	. "github.com/fish-tennis/gserver/internal"
)

type PlayerPacketHandler func(player Player, connection Connection, packet *ProtoPacket)

type PlayerComponentPacketHandler func(component PlayerComponent, connection Connection, packet *ProtoPacket)

// 玩家组件回调接口
type playerComponentHandlerInfo struct {
	componentName string
	method        reflect.Method
	handler       func(c Component, m proto.Message)
}

var _playerComponentHandlerInfos = make(map[PacketCommand]*playerComponentHandlerInfo)

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
// 类似Java的注解功能
func AutoRegisterPlayerComponentProto(packetHandlerRegister PacketHandlerRegister) {
	player := CreateTempPlayer(0, 0)
	for _, component := range player.GetComponents() {
		typ := reflect.TypeOf(component)
		// 如: game.Money -> Money
		componentStructName := typ.String()[strings.LastIndex(typ.String(), ".")+1:]
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if method.Type.NumIn() != 3 {
				continue
			}
			isClientMessage := false
			if strings.HasPrefix(method.Name, "On") {
				// 客户端消息回调
				isClientMessage = true
			} else if strings.HasPrefix(method.Name, "Handle") {
				// 非客户端的逻辑消息回调
			} else {
				continue
			}
			// 消息回调格式: func (this *Money) OnCoinReq(reqCmd PacketCommand, req *pb.CoinReq)
			methonArg1 := method.Type.In(1)
			if methonArg1.Name() != "PacketCommand" {
				continue
			}
			methonArg2 := method.Type.In(2)
			if !strings.HasPrefix(methonArg2.String(), "*pb.") {
				continue
			}
			//if !methonArg2.Implements(reflect.TypeOf(&proto.Message{})) {
			//	continue
			//}
			// 消息名,如: CoinReq
			// *pb.CoinReq -> CoinReq
			messageName := methonArg2.String()[strings.LastIndex(methonArg2.String(), ".")+1:]
			// 客户端消息回调的函数名必须是OnCoinReq
			if isClientMessage && method.Name != fmt.Sprintf("On%v", messageName) {
				logger.Debug("client methodName not match:%v", method.Name)
				continue
			}
			// 非客户端消息回调的函数名必须是HandleCoinReq
			if !isClientMessage && method.Name != fmt.Sprintf("Handle%v", messageName) {
				logger.Debug("methodName not match:%v", method.Name)
				continue
			}
			messageId := util.GetMessageIdByMessageName("gserver", componentStructName, messageName)
			if messageId == 0 {
				continue
			}
			cmd := PacketCommand(messageId)
			// 注册消息回调到组件上
			_playerComponentHandlerInfos[cmd] = &playerComponentHandlerInfo{
				componentName: component.GetName(),
				method: method,
			}
			// 注册客户端消息
			if isClientMessage {
				packetHandlerRegister.Register(cmd, nil, reflect.New(methonArg2.Elem()).Interface().(proto.Message))
			}
			logger.Debug("AutoRegister %v.%v client:%v", componentStructName, method.Name, isClientMessage)
		}
	}
}

// 用于proto_code_gen工具自动生成的消息注册代码
func RegisterPlayerProtoCodeGen(packetHandlerRegister PacketHandlerRegister, componentName string, cmd PacketCommand, protoMessage proto.Message, handler func(c Component, m proto.Message)) {
	_playerComponentHandlerInfos[cmd] = &playerComponentHandlerInfo{
		componentName: componentName,
		handler: handler,
	}
	packetHandlerRegister.Register(cmd, nil, protoMessage)
}
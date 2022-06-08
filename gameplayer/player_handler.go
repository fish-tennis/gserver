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
			if method.Type.NumIn() != 2 || !strings.HasPrefix(method.Name, "On") {
				continue
			}
			// 消息回调格式: func (this *Money) OnCoinReq(req *pb.CoinReq)
			methonArg1 := method.Type.In(1)
			if !strings.HasPrefix(methonArg1.String(), "*pb.") {
				continue
			}
			//if !methonArg1.Implements(reflect.TypeOf(&proto.Message{})) {
			//	continue
			//}
			// 消息名,如: CoinReq
			messageName := methonArg1.String()[strings.LastIndex(methonArg1.String(), ".")+1:]
			// 函数名必须是OnCoinReq
			if method.Name != fmt.Sprintf("On%v", messageName) {
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
			// 注册消息的构造函数
			packetHandlerRegister.Register(cmd, nil, reflect.New(methonArg1.Elem()).Interface().(proto.Message))
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
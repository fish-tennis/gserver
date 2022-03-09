package game

import (
	"fmt"
	. "github.com/fish-tennis/gnet"
	. "github.com/fish-tennis/gserver/internal"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/gameplayer"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

type PlayerPacketHandler func(player gameplayer.Player, connection Connection, packet *ProtoPacket)

type PlayerComponentPacketHandler func(component gameplayer.PlayerComponent, connection Connection, packet *ProtoPacket)

// 客户端连接的handler
type ClientConnectionHandler struct {
	DefaultConnectionHandler
	playerPacketHandlers map[PacketCommand]*packetHandlerInfo
}

type packetHandlerInfo struct {
	componentName string
	method        reflect.Method
}

func NewClientConnectionHandler(protoCodec *ProtoCodec) *ClientConnectionHandler {
	return &ClientConnectionHandler{
		DefaultConnectionHandler: *NewDefaultConnectionHandler(protoCodec),
		playerPacketHandlers:     make(map[PacketCommand]*packetHandlerInfo),
	}
}

func (this *ClientConnectionHandler) OnRecvPacket(connection Connection, packet Packet) {
	if connection.GetTag() != nil {
		// 在线玩家的消息,先检查通过反射注册的回调函数
		player := _gameServer.GetPlayer(connection.GetTag().(int64))
		if player != nil {
			if protoPacket, ok := packet.(*ProtoPacket); ok {
				handlerInfo := this.playerPacketHandlers[protoPacket.Command()]
				if handlerInfo != nil {
					// 在线玩家的消息,自动路由到对应的玩家组件上
					component := player.GetComponent(handlerInfo.componentName)
					if component != nil {
						// 用了反射,性能有所损失
						handlerInfo.method.Func.Call([]reflect.Value{reflect.ValueOf(component), reflect.ValueOf(protoPacket.Message())})
						// 如果有需要保存的数据修改了,即时保存数据库
						player.SaveCache()
						return
					}
				}
			}
		}
	}
	// 未登录的玩家消息和未注册的消息,走默认处理
	// proto_code_gen工具生成的代码,会在这里执行
	this.DefaultConnectionHandler.OnRecvPacket(connection, packet)
}

func (this *ClientConnectionHandler) registerMethod(command PacketCommand, componentName string, method reflect.Method) {
	this.playerPacketHandlers[command] = &packetHandlerInfo{
		componentName: componentName,
		method:        method,
	}
	logger.Debug("registerMethod %v %v.%v", command, componentName, method.Name)
}

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
func (this *ClientConnectionHandler) autoRegisterPlayerComponentProto() {
	player := gameplayer.CreateTempPlayer(0, 0)
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
			this.registerMethod(cmd, component.GetName(), method)
			// 注册消息的构造函数
			this.DefaultConnectionHandler.Register(cmd, nil, func() proto.Message {
				// 用了反射,性能有所损失
				return reflect.New(methonArg1.Elem()).Interface().(proto.Message)
			})
		}
	}
}

// 用于proto_code_gen工具自动生成的消息注册代码
func (this *ClientConnectionHandler) RegisterProtoCodeGen(componentName string, cmd PacketCommand, creator ProtoMessageCreator, handler func(c Component, m proto.Message)) {
	this.Register(cmd, func(connection Connection, packet *ProtoPacket) {
		if connection.GetTag() != nil {
			player := _gameServer.GetPlayer(connection.GetTag().(int64))
			if player != nil {
				// 在线玩家的消息,自动路由到对应的玩家组件上
				component := player.GetComponent(componentName)
				if component != nil {
					handler(component, packet.Message())
					player.SaveCache()
					return
				}
			}
		}
	}, creator)
}

package game

import (
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

type PlayerPacketHandler func(player Player, connection Connection, packet *ProtoPacket)

type PlayerComponentPacketHandler func(component PlayerComponent, connection Connection, packet *ProtoPacket)

// 客户端连接的handler
type ClientConnectionHandler struct {
	gnet.DefaultConnectionHandler
	playerPacketHandlers map[Cmd]*packetHandlerInfo
}

type packetHandlerInfo struct {
	componentName string
	method reflect.Method
}

func NewClientConnectionHandler(protoCodec *gnet.ProtoCodec) *ClientConnectionHandler {
	return &ClientConnectionHandler{
		DefaultConnectionHandler: *gnet.NewDefaultConnectionHandler(protoCodec),
		playerPacketHandlers:     make(map[gnet.PacketCommand]*packetHandlerInfo),
	}
}

func (this* ClientConnectionHandler) OnRecvPacket(connection Connection, packet gnet.Packet) {
	if connection.GetTag() != nil {
		// 在线玩家的消息,自动路由到对应的玩家组件上
		player := gameServer.GetPlayer(connection.GetTag().(int64))
		if player != nil {
			if protoPacket,ok := packet.(*gnet.ProtoPacket); ok {
				handlerInfo := this.playerPacketHandlers[protoPacket.Command()]
				if handlerInfo != nil {
					component := player.GetComponent(handlerInfo.componentName)
					if component != nil {
						// 用了反射,性能有所损失
						handlerInfo.method.Func.Call([]reflect.Value{reflect.ValueOf(component), reflect.ValueOf(protoPacket.Message())})
						// 如果有需要保存的数据修改了,即时保存数据库
						player.Save()
						return
					}
				}
			}
		}
	}
	// 未登录的玩家消息和未注册的消息,走默认处理
	this.DefaultConnectionHandler.OnRecvPacket(connection, packet)
}

func (this* ClientConnectionHandler) RegisterMethod(command Cmd, componentName string, method reflect.Method) {
	this.playerPacketHandlers[command] = &packetHandlerInfo{
		componentName: componentName,
		method: method,
	}
	gnet.LogDebug("RegisterMethod %v %v.%v", command, componentName, method.Name)
}

// 根据proto的命名规则和玩家组件里消息回调的格式,通过反射自动生成消息的注册
func (this* ClientConnectionHandler) autoRegisterPlayerComponentProto() {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	for _,component := range player.components {
		typ := reflect.TypeOf(component)
		// 如: game.Money -> Money
		componentStructName := typ.String()[strings.LastIndex(typ.String(),".")+1:]
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if method.Type.NumIn() != 2 || !strings.HasPrefix(method.Name,"On") {
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
			messageName := methonArg1.String()[strings.LastIndex(methonArg1.String(),".")+1:]
			// 函数名必须是OnCoinReq
			if method.Name != fmt.Sprintf("On%v", messageName) {
				gnet.LogDebug("methodName not match:%v", method.Name)
				continue
			}
			//// proto文件里定义的package名是组件名的小写, 如package money
			//// enum Message的全名:money.CmdMoney
			//enumTypeName := fmt.Sprintf("%v.Cmd%v", strings.ToLower(componentStructName), componentStructName)
			//enumTyp,err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(enumTypeName))
			//if err != nil {
			//	gnet.LogDebug("%v err:%v", enumTypeName, err)
			//	continue
			//}
			//// 如: Cmd_CoinReq
			//enumIdName := fmt.Sprintf("Cmd_%v", messageName)
			//enumNumber := enumTyp.Descriptor().Values().ByName(protoreflect.Name(enumIdName)).Number()
			//gnet.LogDebug("enum %v:%v", enumIdName, enumNumber)
			messageId := util.GetMessageIdByMessageName(componentStructName, messageName)
			if messageId == 0 {
				continue
			}
			cmd := gnet.PacketCommand(messageId)
			// 注册消息回调到组件上
			this.RegisterMethod(cmd, component.GetName(), method)
			// 注册消息的构造函数
			this.DefaultConnectionHandler.Register(cmd, nil, func() proto.Message {
				// 用了反射,性能有所损失
				return reflect.New(methonArg1.Elem()).Interface().(proto.Message)
			})
		}
	}
}

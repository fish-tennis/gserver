package game

import (
	"fmt"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/pb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"reflect"
	"strings"
)

type PlayerPacketHandler func(player Player, connection gnet.Connection, packet *gnet.ProtoPacket)

type PlayerComponentPacketHandler func(component PlayerComponent, connection gnet.Connection, packet *gnet.ProtoPacket)

// 客户端连接的handler
type ClientConnectionHandler struct {
	gnet.DefaultConnectionHandler
	playerPacketHandlers map[gnet.PacketCommand]*packetHandlerInfo
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

func (this* ClientConnectionHandler) OnRecvPacket(connection gnet.Connection, packet gnet.Packet) {
	if connection.GetTag() != nil {
		// 在线玩家的消息,自动路由到对应的玩家组件上
		player := gameServer.GetPlayer(connection.GetTag().(int64))
		if player != nil {
			if protoPacket,ok := packet.(*gnet.ProtoPacket); ok {
				handlerInfo := this.playerPacketHandlers[protoPacket.Command()]
				if handlerInfo != nil {
					component := player.GetComponentByName(handlerInfo.componentName)
					if component != nil {
						// 用了反射,性能有所损耗
						handlerInfo.method.Func.Call([]reflect.Value{reflect.ValueOf(component), reflect.ValueOf(protoPacket.Message())})
						return
					}
				}
			}
		}
	}
	// 未登录的玩家消息和未注册的消息,走默认处理
	this.DefaultConnectionHandler.OnRecvPacket(connection, packet)
}

func (this* ClientConnectionHandler) RegisterMethod(command gnet.PacketCommand, componentName string, method reflect.Method) {
	this.playerPacketHandlers[command] = &packetHandlerInfo{
		componentName: componentName,
		method: method,
	}
	gnet.LogDebug("RegisterMethod %v %v.%v", command, componentName, method.Name)
}

func (this* ClientConnectionHandler) autoRegister() {
	playerData := &pb.PlayerData{}
	player := CreatePlayerFromData(playerData)
	for _,component := range player.components {
		typ := reflect.TypeOf(component)
		//gnet.LogDebug("component type:%v", typ)
		//gnet.LogDebug("component typename:%v", typ.Name())
		packageName := typ.String()[strings.LastIndex(typ.String(),".")+1:]
		//packageName = strings.ToLower(packageName)
		//gnet.LogDebug("component packageName:%v", packageName)
		for i := 0; i < typ.NumMethod(); i++ {
			method := typ.Method(i)
			if method.Type.NumIn() != 2 || !strings.HasPrefix(method.Name,"On") {
				continue
			}
			methonArg1 := method.Type.In(1)
			if !strings.HasPrefix(methonArg1.String(), "*pb.") {
				continue
			}
			//if !methonArg1.Implements(reflect.TypeOf(&proto.Message{})) {
			//	continue
			//}
			//gnet.LogDebug("methonArg1:%v string:%v", methonArg1, methonArg1.String())
			//gnet.LogDebug("PkgPath:%v", methonArg1.PkgPath())
			// 根据规则,自动找到消息号
			messageName := methonArg1.String()[strings.LastIndex(methonArg1.String(),".")+1:]
			enumTypeName := fmt.Sprintf("%v.Cmd%v", strings.ToLower(packageName), packageName)
			enumTyp,err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(enumTypeName))
			if err != nil {
				gnet.LogDebug("err:%v", err)
				continue
			}
			enumIdName := fmt.Sprintf("Cmd_%v", messageName)
			enumNumber := enumTyp.Descriptor().Values().ByName(protoreflect.Name(enumIdName)).Number()
			//gnet.LogDebug("enum %v:%v", enumIdName, enumNumber)
			cmd := gnet.PacketCommand(enumNumber)
			this.RegisterMethod(cmd, component.GetName(), method)
			this.DefaultConnectionHandler.Register(cmd, nil, func() proto.Message {
				return reflect.New(methonArg1.Elem()).Interface().(proto.Message)
			})
		}
	}
}

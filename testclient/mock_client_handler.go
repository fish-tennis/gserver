package testclient

import (
	"fmt"
	. "github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/logger"
	"github.com/fish-tennis/gserver/pb"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"reflect"
	"strings"
)

type MockClientHandler struct {
	DefaultConnectionHandler
	methods map[PacketCommand]reflect.Method
}

func NewMockClientHandler(protoCodec *ProtoCodec) *MockClientHandler {
	handler := &MockClientHandler{
		DefaultConnectionHandler: *NewDefaultConnectionHandler(protoCodec),
		methods:                  make(map[PacketCommand]reflect.Method),
	}
	handler.RegisterHeartBeat(PacketCommand(pb.CmdInner_Cmd_HeartBeatReq), func() proto.Message {
		return &pb.HeartBeatReq{
			Timestamp: uint64(util.GetCurrentMS()),
		}
	})
	handler.Register(PacketCommand(pb.CmdInner_Cmd_HeartBeatRes), func(connection Connection, packet *ProtoPacket) {
	} , new(pb.HeartBeatRes))
	handler.SetUnRegisterHandler(func(connection Connection, packet *ProtoPacket) {
		logger.Debug("un register %v", string(packet.Message().ProtoReflect().Descriptor().Name()))
	})
	return handler
}

func (this *MockClientHandler) OnRecvPacket(connection Connection, packet Packet) {
	if connection.GetTag() != nil {
		accountName := connection.GetTag().(string)
		mockClient := _testClient.getMockClientByAccountName(accountName)
		if mockClient == nil {
			return
		}
		if protoPacket, ok := packet.(*ProtoPacket); ok {
			handlerMethod, ok2 := this.methods[protoPacket.Command()]
			if ok2 {
				handlerMethod.Func.Call([]reflect.Value{reflect.ValueOf(mockClient), reflect.ValueOf(protoPacket.Message())})
				return
			}
		}
		this.DefaultConnectionHandler.OnRecvPacket(connection, packet)
	}
}

// 通过反射自动注册消息回调
func (this *MockClientHandler) autoRegister() {
	typ := reflect.TypeOf(&MockClient{})
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		// func (this *MockClient) OnMessageName(res *pb.MessageName)
		if method.Type.NumIn() != 2 || !strings.HasPrefix(method.Name, "On") {
			continue
		}
		methonArg1 := method.Type.In(1)
		if !strings.HasPrefix(methonArg1.String(), "*pb.") {
			continue
		}
		// 消息名,如: LoginRes
		messageName := methonArg1.String()[strings.LastIndex(methonArg1.String(), ".")+1:]
		// 函数名必须是onLoginRes
		if method.Name != fmt.Sprintf("On%v", messageName) {
			logger.Debug("methodName not match:%v", method.Name)
			continue
		}
		// Cmd_LoginRes
		enumValueName := fmt.Sprintf("Cmd_%v", messageName)
		var messageId int32
		protoregistry.GlobalTypes.RangeEnums(func(enumType protoreflect.EnumType) bool {
			// gserver.Login.CmdLogin
			enumValueDescriptor := enumType.Descriptor().Values().ByName(protoreflect.Name(enumValueName))
			if enumValueDescriptor != nil {
				messageId = int32(enumValueDescriptor.Number())
				return false
			}
			return true
		})
		if messageId == 0 {
			continue
		}
		cmd := PacketCommand(messageId)
		// 注册消息回调到组件上
		this.methods[cmd] = method
		// 注册消息的构造函数
		this.DefaultConnectionHandler.Register(cmd, nil, reflect.New(methonArg1.Elem()).Interface().(proto.Message))
		logger.Debug("register %v %v", messageId, method.Name)
	}
}

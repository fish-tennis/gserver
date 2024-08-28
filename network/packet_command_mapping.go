package network

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoimpl"
	"log/slog"
	"reflect"
	"strings"
)

const (
	// 对应proto文件里的package导出名
	ProtoPackageName = "gserver"
	// 客户端和服务器之间的消息号定义
	CmdClientEnumName = "CmdClient" // 对应proto/cmd_client.proto里的enum CmdClient
	// 服务器之间的消息号定义
	CmdServerEnumName = "CmdServer" // 对应proto/cmd_server.proto里的enum CmdServer
)

var (
	// reflect.TypeOf(*pb.Xxx).Elem() -> packetCommand
	messageTypeCmdMapping = make(map[reflect.Type]int32)
)

func init() {
	initCommandMapping()
}

// 本项目的request和response的消息号规范: resCmd = reqCmd + 1
func GetResCommand(reqCommand int32) int32 {
	return reqCommand + 1
}

func GetCommandByProto(protoMessage proto.Message) int32 {
	typ := reflect.TypeOf(protoMessage)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if cmd, ok := messageTypeCmdMapping[typ]; ok {
		return cmd
	}
	slog.Warn("GetCommandByProtoErr", "messageName", proto.MessageName(protoMessage))
	return 0
}

// 注册消息和消息号的映射关系
func initCommandMapping() {
	for _, cmdEnumName := range []string{CmdClientEnumName, CmdServerEnumName} {
		cmdEnumName = fmt.Sprintf("%v.%v", ProtoPackageName, cmdEnumName)
		enumType, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(cmdEnumName))
		if err != nil {
			panic(fmt.Sprintf("%v not found", cmdEnumName))
		}
		enumDescs := enumType.Descriptor().Values()
		for i := 0; i < enumDescs.Len(); i++ {
			enumValueDesc := enumDescs.Get(i)
			messageId := enumValueDesc.Number()
			if messageId == 0 {
				continue // CmdClient_None
			}
			messageName := strings.TrimPrefix(string(enumValueDesc.Name()), "Cmd_")
			fullMessageName := fmt.Sprintf("%v.%v", ProtoPackageName, messageName)
			messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(fullMessageName))
			if err != nil {
				slog.Warn("FindMessageByNameErr", "messageName", messageName, "id", messageId, "err", err)
				continue
			}
			if messageInfo, ok := messageType.(*protoimpl.MessageInfo); ok {
				typ := messageInfo.GoReflectType.Elem()
				messageTypeCmdMapping[typ] = int32(messageId)
				slog.Info("CommandMapping", "messageName", messageName, "id", messageId)
			}
		}
	}
}

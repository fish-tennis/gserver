package network

import (
	"encoding/json"
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoimpl"
	"log/slog"
	"os"
	"reflect"
	"strings"
)

const (
	// 对应proto文件里的package导出名
	ProtoPackageName = "gserver"
)

var (
	// reflect.TypeOf(*pb.Xxx).Elem() -> packetCommand
	_messageTypeCmdMapping = make(map[reflect.Type]int32)
	_cmdMessageNameMapping = make(map[int32]string)
)

// 本项目的request和response的消息号规范: XxxReq XxxRes
func GetResCommand(reqCommand int32) int32 {
	reqMessageName, ok := _cmdMessageNameMapping[reqCommand]
	if !ok {
		slog.Warn("GetResCommandErr", "reqCommand", reqCommand)
		return 0
	}
	if !strings.HasSuffix(reqMessageName, "Req") {
		return 0
	}
	resMessageName := strings.TrimSuffix(reqMessageName, "Req") + "Res"
	fullMessageName := GetFullMessageName(ProtoPackageName, resMessageName)
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(fullMessageName))
	if err != nil {
		slog.Warn("GetResCommandErr", "resMessageName", resMessageName, "reqCommand", reqCommand, "err", err)
		return 0
	}
	if messageInfo, ok := messageType.(*protoimpl.MessageInfo); ok {
		typ := messageInfo.GoReflectType.Elem()
		return _messageTypeCmdMapping[typ]
	}
	return 0
}

func GetCommandByProto(protoMessage proto.Message) int32 {
	typ := reflect.TypeOf(protoMessage)
	if typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if cmd, ok := _messageTypeCmdMapping[typ]; ok {
		return cmd
	}
	slog.Warn("GetCommandByProtoErr", "messageName", proto.MessageName(protoMessage))
	return 0
}

func InitCommandMappingFromFile(file string) {
	mapping := loadCommandMapping(file)
	for messageName, messageId := range mapping {
		fullMessageName := GetFullMessageName(ProtoPackageName, messageName)
		messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(fullMessageName))
		if err != nil {
			slog.Warn("FindMessageByNameErr", "messageName", messageName, "id", messageId, "err", err)
			continue
		}
		if messageInfo, ok := messageType.(*protoimpl.MessageInfo); ok {
			typ := messageInfo.GoReflectType.Elem()
			_messageTypeCmdMapping[typ] = int32(messageId)
			_cmdMessageNameMapping[int32(messageId)] = messageName
			slog.Info("CommandMapping", "name", messageName, "id", messageId)
		}
	}
}

func loadCommandMapping(fileName string) map[string]int {
	mapping := make(map[string]int)
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("loadCommandMappingErr", "fileName", fileName, "err", err)
		return mapping
	}
	err = json.Unmarshal(fileData, &mapping)
	if err != nil {
		slog.Error("loadCommandMappingErr", "fileName", fileName, "err", err)
		return mapping
	}
	return mapping
}

func NewMessageByName(messageName string) proto.Message {
	fullMessageName := GetFullMessageName(ProtoPackageName, messageName)
	messageType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(fullMessageName))
	if err != nil {
		slog.Error("newMessageByNameErr", "messageName", messageName, "err", err)
		return nil
	}
	if messageInfo, ok := messageType.(*protoimpl.MessageInfo); ok {
		typ := messageInfo.GoReflectType.Elem()
		return reflect.New(typ).Interface().(proto.Message)
	}
	return nil
}

func GetFullMessageName(packageName, messageName string) string {
	if ProtoPackageName == "" {
		return messageName
	}
	return ProtoPackageName + "." + messageName
}

func RegisterPacketHandler(register gnet.PacketHandlerRegister, protoMessage proto.Message, handler gnet.PacketHandler) {
	cmd := GetCommandByProto(protoMessage)
	if cmd == 0 {
		slog.Warn("RegisterPacketHandlerErr", "protoMessage", protoMessage)
		return
	}
	register.Register(gnet.PacketCommand(uint16(cmd)), handler, protoMessage)
}

package internal

import (
	"fmt"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"strings"
)

const (
	// 客户端和服务器之间的消息号定义
	CmdClientEnumName = "CmdClient" // 对应proto/cmd_client.proto里的enum CmdClient
	// 服务器之间的消息号定义
	CmdServerEnumName = "CmdServer" // 对应proto/cmd_server.proto里的enum CmdServer
	// 内部消息号定义
	CmdInnerEnumName = "CmdInner" // 对应proto/inner.proto里的enum CmdInner
)

// 本项目的request和response的消息号规范: resCmd = reqCmd + 1
func GetResCommand(reqCommand int32) int32 {
	return reqCommand + 1
}

func GetClientCommandByProto(protoMessage proto.Message) int32 {
	return getCommandByProto(CmdClientEnumName, protoMessage)
}

func GetServerCommandByProto(protoMessage proto.Message) int32 {
	return getCommandByProto(CmdServerEnumName, protoMessage)
}

func GetCommandByProto(protoMessage proto.Message) int32 {
	cmd := GetClientCommandByProto(protoMessage)
	if cmd != 0 {
		return cmd
	}
	return GetServerCommandByProto(protoMessage)
}

func getCommandByProto(cmdEnumName string, protoMessage proto.Message) int32 {
	// CmdClient或CmdServer或CmdInner
	cmdEnumName = fmt.Sprintf("%v.%v", ProtoPackageName, cmdEnumName)
	enumType, err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(cmdEnumName))
	if err != nil {
		//gentity.Debug("%v err:%v", enumTypeName, err)
		return 0
	}
	// 如: pb.FinishQuestReq -> Cmd_FinishQuestReq
	enumIdName := GetEnumValueNameOfProtoMessage(protoMessage)
	enumValue := enumType.Descriptor().Values().ByName(protoreflect.Name(enumIdName))
	if enumValue == nil {
		return 0
	}
	enumNumber := enumValue.Number()
	//logger.Debug("enum %v:%v", enumIdName, enumNumber)
	return int32(enumNumber)
}

func GetShortNameOfProtoMessage(protoMessage proto.Message) string {
	messageName := string(proto.MessageName(protoMessage).Name())
	// 消息名,如: FinishQuestReq
	// *pb.FinishQuestReq -> FinishQuestReq
	idx := strings.LastIndex(messageName, ".")
	if idx < 0 {
		return messageName
	}
	return messageName[idx+1:]
}

// proto.Message对应的消息号枚举值名
// 本项目的消息号的规范: Cmd_MessageName
func GetEnumValueNameOfProtoMessage(protoMessage proto.Message) string {
	messageName := GetShortNameOfProtoMessage(protoMessage)
	return fmt.Sprintf("Cmd_%v", messageName)
}

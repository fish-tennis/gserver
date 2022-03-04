package util

import (
	"fmt"
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

// 根据组件名和消息名获取对应的消息号
func GetMessageIdByMessageName(componentStructName, messageName string) int32 {
	// proto文件里定义的package名是组件名的小写, 如package money
	// enum Message的全名:money.CmdMoney
	enumTypeName := fmt.Sprintf("%v.Cmd%v", "gserver", componentStructName)
	enumTyp,err := protoregistry.GlobalTypes.FindEnumByName(protoreflect.FullName(enumTypeName))
	if err != nil {
		logger.Debug("%v err:%v", enumTypeName, err)
		return 0
	}
	// 如: Cmd_CoinReq
	enumIdName := fmt.Sprintf("Cmd_%v", messageName)
	enumNumber := enumTyp.Descriptor().Values().ByName(protoreflect.Name(enumIdName)).Number()
	//logger.Debug("enum %v:%v", enumIdName, enumNumber)
	return int32(enumNumber)
}

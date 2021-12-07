package tool

import (
	"fmt"
	"github.com/fish-tennis/gserver/util"
	"google.golang.org/protobuf/reflect/protoregistry"
	"reflect"
	"strings"
)

func autoGenerator( playerComponent interface{}) {
	var builder strings.Builder
	builder.WriteString("import \"github.com/fish-tennis/gserver/pb\"")
	builder.WriteString("\n\n")
	playerComponentType := reflect.TypeOf(playerComponent)
	fmt.Println(playerComponentType)
	componentStructName := playerComponentType.String()[strings.LastIndex(playerComponentType.String(),".")+1:]
	fmt.Println(componentStructName)
	packageName := strings.ToLower(componentStructName)
	path := fmt.Sprintf("%v.proto", packageName)
	fileDescriptor,_ := protoregistry.GlobalFiles.FindFileByPath(path)
	fmt.Println(fmt.Sprintf("%v",fileDescriptor))
	msgDescriptors := fileDescriptor.Messages()
	for i := 0; i < msgDescriptors.Len(); i++ {
		msgDescriptor := msgDescriptors.Get(i)
		fmt.Println(fmt.Sprintf("%v",msgDescriptor))
		messageName := msgDescriptor.Name()
		// 必须有对应的消息号枚举
		if util.GetMessageIdByMessageName(componentStructName, string(messageName)) == 0 {
			continue
		}
		messageType,err := protoregistry.GlobalTypes.FindMessageByName(msgDescriptor.FullName())
		if err != nil || messageType == nil {
			continue
		}
		message := messageType.New()
		if message == nil {
			continue
		}
		//println(fmt.Sprintf("1:%v", message))
		//println(fmt.Sprintf("2:%v", message.Interface()))
		//println(fmt.Sprintf("3:%v", reflect.TypeOf(message.Interface())))
		//println(fmt.Sprintf("4:%v", reflect.TypeOf(message.Interface()).Elem()))
		typ := reflect.TypeOf(message.Interface()).Elem()
		if typ.NumField() < 4 {
			continue
		}
		field := typ.Field(3)
		//println(fmt.Sprintf("field:%v", field))
		if field.Tag.Get("send") != "Player" {
			continue
		}
		cmdName := fmt.Sprintf("Cmd%v_Cmd_%v", componentStructName, messageName)
		builder.WriteString(playerSendTemplate(cmdName, string(messageName)))
		builder.WriteString("\n")
	}
	enumDescriptors := fileDescriptor.Enums()
	for i := 0; i < enumDescriptors.Len(); i++ {
		enumDescriptor := enumDescriptors.Get(i)
		fmt.Println(fmt.Sprintf("%v",enumDescriptor))
	}
	//protoregistry.GlobalFiles.RangeFiles(func(descriptor protoreflect.FileDescriptor) bool {
	//	fmt.Println(fmt.Sprintf("%v",descriptor))
	//	return true
	//})
	fmt.Println(builder.String())
}

func playerSendTemplate(cmdName, messageName string) string {
	return fmt.Sprintf("func (this *Player) Send%v(packet *pb.%v) bool {\n\treturn this.Send(Cmd(pb.%v), packet)\n}\n",
		messageName, messageName, cmdName)
}
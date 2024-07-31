package internal

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gentity/util"
	"github.com/fish-tennis/gnet"
	"google.golang.org/protobuf/proto"
	"reflect"
	"strings"
)

// 消息回调接口信息
type PacketHandlerInfo struct {
	// 组件名,如果为空,就表示是直接写在Entity上的接口
	ComponentName string
	// 消息号
	Cmd gnet.PacketCommand
	// 函数信息
	Method reflect.Method
	// 匿名函数接口
	Handler func(c gentity.Component, m proto.Message)
}

// 消息回调接口管理类
type PacketHandlerMgr struct {
	HandlerInfos map[gnet.PacketCommand]*PacketHandlerInfo
}

func NewPacketHandlerMgr() *PacketHandlerMgr {
	return &PacketHandlerMgr{
		HandlerInfos: make(map[gnet.PacketCommand]*PacketHandlerInfo),
	}
}

// 注册消息回调
func (this *PacketHandlerMgr) AddHandlerInfo(handlerInfo *PacketHandlerInfo) {
	if oldInfo, ok := this.HandlerInfos[handlerInfo.Cmd]; ok {
		if oldInfo.ComponentName != handlerInfo.ComponentName || oldInfo.Method.Name != handlerInfo.Method.Name {
			gentity.GetLogger().Error("duplicate cmd:%v component:%v method:%v", handlerInfo.Cmd, oldInfo.ComponentName, oldInfo.Method.Name)
		}
	}
	this.HandlerInfos[handlerInfo.Cmd] = handlerInfo
}

// 自动注册消息回调接口类型是func (this *Component) OnFinishQuestReq(cmd PacketCommand, req *pb.XxxMessage)的回调
//
//	根据proto的命名规则和组件里消息回调的格式,通过反射自动生成消息的注册
//	类似Java的注解功能
//	用于服务器内部的逻辑消息
//	可以在组件里编写函数: HandleXxx(cmd PacketCommand, req *pb.Xxx)
func (this *PacketHandlerMgr) AutoRegister(entity gentity.Entity, methodNamePrefix, protoPackageName string) {
	// 扫描entity上的消息回调接口
	this.scanMethods(entity, nil, "", methodNamePrefix, protoPackageName)
	// 扫描entity的组件上的消息回调接口
	entity.RangeComponent(func(component gentity.Component) bool {
		this.scanMethods(component, nil, "", methodNamePrefix, protoPackageName)
		return true
	})
}

// 自动注册消息回调接口类型是func (this *Component) OnFinishQuestReq(cmd PacketCommand, req *pb.XxxMessage)的回调,并注册PacketHandler,一般用于服务器监听客户端的连接
//
//	根据proto的命名规则和消息回调的格式,通过反射自动生成消息的注册
//	类似Java的注解功能
//	游戏常见有2类消息
//	1.客户端的请求消息
//	可以在组件里编写函数: OnXxx(cmd PacketCommand, req *pb.Xxx)
//	2.服务器内部的逻辑消息
//	可以在组件里编写函数: HandleXxx(cmd PacketCommand, req *pb.Xxx)
func (this *PacketHandlerMgr) AutoRegisterWithClient(entity gentity.Entity, packetHandlerRegister gnet.PacketHandlerRegister, clientHandlerPrefix, otherHandlerPrefix, protoPackageName string) {
	// 扫描entity上的消息回调接口
	this.scanMethods(entity, packetHandlerRegister, clientHandlerPrefix, otherHandlerPrefix, protoPackageName)
	// 扫描entity的组件上的消息回调接口
	entity.RangeComponent(func(component gentity.Component) bool {
		this.scanMethods(component, packetHandlerRegister, clientHandlerPrefix, otherHandlerPrefix, protoPackageName)
		return true
	})
}

// 扫描一个struct的函数
func (this *PacketHandlerMgr) scanMethods(obj any, packetHandlerRegister gnet.PacketHandlerRegister,
	clientHandlerPrefix, otherHandlerPrefix, protoPackageName string) {
	typ := reflect.TypeOf(obj)
	componentName := ""
	component, ok := obj.(gentity.Component)
	if ok {
		componentName = component.GetName()
	}
	// 如: game.Quest -> Quest
	componentStructName := typ.String()[strings.LastIndex(typ.String(), ".")+1:]
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		if method.Type.NumIn() != 3 {
			continue
		}
		isClientMessage := false
		if packetHandlerRegister != nil && clientHandlerPrefix != "" && strings.HasPrefix(method.Name, clientHandlerPrefix) {
			// 客户端消息回调
			isClientMessage = true
		} else if otherHandlerPrefix != "" && strings.HasPrefix(method.Name, otherHandlerPrefix) {
			// 非客户端的逻辑消息回调
		} else {
			continue
		}
		// 消息回调格式: func (this *Quest) OnFinishQuestReq(cmd PacketCommand, req *pb.FinishQuestReq)
		methodArg1 := method.Type.In(1)
		// 参数1是消息号
		if methodArg1.Name() != "PacketCommand" && methodArg1.Name() != "gnet.PacketCommand" {
			continue
		}
		methodArg2 := method.Type.In(2)
		// 参数2是proto定义的消息体
		if !methodArg2.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			continue
		}
		// 消息名,如: FinishQuestReq
		// *pb.FinishQuestReq -> FinishQuestReq
		messageName := methodArg2.String()[strings.LastIndex(methodArg2.String(), ".")+1:]
		// 客户端消息回调的函数名规则,如OnFinishQuestReq
		if isClientMessage && method.Name != fmt.Sprintf("%v%v", clientHandlerPrefix, messageName) {
			gentity.GetLogger().Debug("client methodName not match:%v", method.Name)
			continue
		}
		// 非客户端消息回调的函数名规则,如HandleFinishQuestReq
		if !isClientMessage && method.Name != fmt.Sprintf("%v%v", otherHandlerPrefix, messageName) {
			gentity.GetLogger().Debug("methodName not match:%v", method.Name)
			continue
		}
		messageId := util.GetMessageIdByMessageName(protoPackageName, componentStructName, messageName)
		if messageId == 0 {
			gentity.GetLogger().Debug("methodName match:%v but messageId==0", method.Name)
			continue
		}
		cmd := gnet.PacketCommand(messageId)
		this.AddHandlerInfo(&PacketHandlerInfo{
			ComponentName: componentName,
			Cmd:           cmd,
			Method:        method,
		})
		// 注册客户端消息
		if isClientMessage && packetHandlerRegister != nil {
			packetHandlerRegister.Register(cmd, nil, reflect.New(methodArg2.Elem()).Interface().(proto.Message))
		}
		gentity.GetLogger().Debug("ScanPacketHandler %v.%v %v client:%v", componentStructName, method.Name, messageId, isClientMessage)
	}
}

// 用于proto_code_gen工具自动生成的消息注册代码
func (this *PacketHandlerMgr) RegisterProtoCodeGen(packetHandlerRegister gnet.PacketHandlerRegister, componentName string, cmd gnet.PacketCommand, protoMessage proto.Message, handler func(c gentity.Component, m proto.Message)) {
	this.HandlerInfos[cmd] = &PacketHandlerInfo{
		ComponentName: componentName,
		Cmd:           cmd,
		Handler:       handler,
	}
	packetHandlerRegister.Register(cmd, nil, protoMessage)
}

// 执行注册的消息回调接口
// return true表示执行了接口
// return false表示未执行
func (this *PacketHandlerMgr) Invoke(entity gentity.Entity, packet gnet.Packet) bool {
	// 先找组件接口
	handlerInfo := this.HandlerInfos[packet.Command()]
	if handlerInfo != nil {
		if handlerInfo.ComponentName != "" {
			component := entity.GetComponentByName(handlerInfo.ComponentName)
			if component != nil {
				if handlerInfo.Handler != nil {
					handlerInfo.Handler(component, packet.Message())
					return true
				} else {
					if !handlerInfo.Method.Func.IsValid() {
						gentity.GetLogger().Error("InvokeErr method invalid cmd:%v", packet.Command())
						return false
					}
					// 反射调用函数
					handlerInfo.Method.Func.Call([]reflect.Value{reflect.ValueOf(component),
						reflect.ValueOf(packet.Command()),
						reflect.ValueOf(packet.Message())})
					return true
				}
			}
		} else {
			if !handlerInfo.Method.Func.IsValid() {
				gentity.GetLogger().Error("InvokeErr method invalid cmd:%v", packet.Command())
				return false
			}
			// 反射调用函数
			handlerInfo.Method.Func.Call([]reflect.Value{reflect.ValueOf(entity),
				reflect.ValueOf(packet.Command()),
				reflect.ValueOf(packet.Message())})
			return true
		}
	}
	return false
}

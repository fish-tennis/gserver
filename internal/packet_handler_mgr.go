package internal

import (
	"fmt"
	"github.com/fish-tennis/gentity"
	"github.com/fish-tennis/gnet"
	"github.com/fish-tennis/gserver/network"
	"google.golang.org/protobuf/proto"
	"log/slog"
	"reflect"
	"strings"
)

// 消息回调接口信息
type PacketHandlerInfo struct {
	// 组件名,如果为空,就表示是直接写在Entity上的接口
	ComponentName string
	// req消息号
	Cmd gnet.PacketCommand
	// res消息号
	ResCmd gnet.PacketCommand
	// res消息type
	ResMessageElem reflect.Type
	// 函数信息
	Method reflect.Method
}

// 消息回调接口管理类
type PacketHandlerMgr struct {
	HandlerInfos map[gnet.PacketCommand]*PacketHandlerInfo
	// TODO: 消息结构类型和回调函数的映射
	//HandlerByTyp map[reflect.Type]*PacketHandlerInfo
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
			slog.Error("duplicate cmd", "cmd", handlerInfo.Cmd, "component", oldInfo.ComponentName, "method", oldInfo.Method.Name)
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
func (this *PacketHandlerMgr) AutoRegister(entity gentity.Entity, methodNamePrefix string) {
	// 扫描entity上的消息回调接口
	this.scanMethods(entity, nil, "", methodNamePrefix)
	// 扫描entity的组件上的消息回调接口
	entity.RangeComponent(func(component gentity.Component) bool {
		this.scanMethods(component, nil, "", methodNamePrefix)
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
func (this *PacketHandlerMgr) AutoRegisterWithClient(entity gentity.Entity, packetHandlerRegister gnet.PacketHandlerRegister, clientHandlerPrefix, otherHandlerPrefix string) {
	// 扫描entity上的消息回调接口
	this.scanMethods(entity, packetHandlerRegister, clientHandlerPrefix, otherHandlerPrefix)
	// 扫描entity的组件上的消息回调接口
	entity.RangeComponent(func(component gentity.Component) bool {
		this.scanMethods(component, packetHandlerRegister, clientHandlerPrefix, otherHandlerPrefix)
		return true
	})
}

// 扫描一个struct的函数
func (this *PacketHandlerMgr) scanMethods(obj any, packetHandlerRegister gnet.PacketHandlerRegister,
	clientHandlerPrefix, otherHandlerPrefix string) {
	typ := reflect.TypeOf(obj)
	componentName := ""
	component, ok := obj.(gentity.Component)
	if ok {
		componentName = component.GetName()
	}
	for i := 0; i < typ.NumMethod(); i++ {
		method := typ.Method(i)
		if !method.IsExported() {
			continue
		}
		// func (c *Component) OnXxxReq(req *pb.XxxReq)
		// func (c *Component) OnXxxReq(req *pb.XxxReq) (*pb.XxxRes,error)
		if method.Type.NumIn() != 2 {
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
		reqArg := method.Type.In(1)
		// 参数1是proto定义的消息体
		if !reqArg.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
			continue
		}
		// 消息名,如: FinishQuestReq
		// *pb.FinishQuestReq -> FinishQuestReq
		messageName := reqArg.String()[strings.LastIndex(reqArg.String(), ".")+1:]
		// 客户端消息回调的函数名规则,如OnFinishQuestReq
		if isClientMessage && method.Name != fmt.Sprintf("%v%v", clientHandlerPrefix, messageName) {
			slog.Debug("client methodName not match", "method", method.Name)
			continue
		}
		// 非客户端消息回调的函数名规则,如HandleFinishQuestReq
		if !isClientMessage && method.Name != fmt.Sprintf("%v%v", otherHandlerPrefix, messageName) {
			slog.Debug("methodName not match", "method", method.Name)
			continue
		}
		messageId := network.GetCommandByProto(reflect.New(reqArg.Elem()).Interface().(proto.Message))
		if messageId == 0 {
			slog.Debug("messageId==0", "method", method.Name, "messageName", messageName)
			continue
		}
		var (
			resCmd         int32
			resMessageElem reflect.Type
		)
		//if method.Type.NumOut() < 2 {
		//	slog.Debug("len(returnValues)<2", "method", method.Name)
		//	continue
		//}
		// func (c *Component) OnXxxReq(req *pb.XxxReq) (*pb.XxxRes,error)
		if method.Type.NumOut() == 2 {
			resArg := method.Type.Out(0)
			// 返回值1是proto定义的消息体
			if !resArg.Implements(reflect.TypeOf((*proto.Message)(nil)).Elem()) {
				slog.Debug("resArg not proto.Message", "method", method.Name)
				continue
			}
			resMessageElem = resArg.Elem()
			resCmd = network.GetCommandByProto(reflect.New(resMessageElem).Interface().(proto.Message))
			if resCmd == 0 {
				slog.Debug("resCmd==0", "method", method.Name)
				continue
			}
		}
		reqCmd := gnet.PacketCommand(messageId)
		this.AddHandlerInfo(&PacketHandlerInfo{
			ComponentName:  componentName,
			Cmd:            reqCmd,
			ResCmd:         gnet.PacketCommand(resCmd),
			ResMessageElem: resMessageElem,
			Method:         method,
		})
		// 注册客户端消息
		if isClientMessage && packetHandlerRegister != nil {
			packetHandlerRegister.Register(reqCmd, nil, reflect.New(reqArg.Elem()).Interface().(proto.Message))
		}
		slog.Debug("scanMethods", "component", componentName, "method", method.Name, "reqCmd", messageId, "isClient", isClientMessage)
	}
}

// 执行注册的消息回调接口
// return true表示执行了接口
// return false表示未执行
func (this *PacketHandlerMgr) Invoke(entity gentity.Entity, packet gnet.Packet, processReturnValues func(*PacketHandlerInfo, []reflect.Value)) bool {
	// 先找组件接口
	handlerInfo := this.HandlerInfos[packet.Command()]
	if handlerInfo != nil {
		if handlerInfo.ComponentName != "" {
			component := entity.GetComponentByName(handlerInfo.ComponentName)
			if component != nil {
				if !handlerInfo.Method.Func.IsValid() {
					slog.Error("InvokeErr method invalid", "cmd", packet.Command())
					return false
				}
				// 反射调用函数 func (q *Quest) OnFinishQuestReq(cmd PacketCommand, req *pb.FinishQuestReq)
				returnValues := handlerInfo.Method.Func.Call([]reflect.Value{reflect.ValueOf(component),
					reflect.ValueOf(packet.Message())})
				if len(returnValues) > 0 && processReturnValues != nil && handlerInfo.ResCmd > 0 {
					processReturnValues(handlerInfo, returnValues)
				}
				return true
			}
		} else {
			if !handlerInfo.Method.Func.IsValid() {
				slog.Error("InvokeErr method invalid", "cmd", packet.Command())
				return false
			}
			// 反射调用函数
			returnValues := handlerInfo.Method.Func.Call([]reflect.Value{reflect.ValueOf(entity),
				reflect.ValueOf(packet.Message())})
			if len(returnValues) > 0 && processReturnValues != nil && handlerInfo.ResCmd > 0 {
				processReturnValues(handlerInfo, returnValues)
			}
			return true
		}
	}
	return false
}

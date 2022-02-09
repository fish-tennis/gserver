package internal

import (
	"github.com/fish-tennis/gserver/logger"
	"google.golang.org/protobuf/proto"
)

// 实体接口
type Entity interface {
	// 查找某个组件
	GetComponent(componentName string) Component

	// 组件列表
	GetComponents() []Component
}

// 实体组件接口
type Component interface {
	//GetId() int

	// 组件名
	GetName() string
	GetNameLower() string

	// 所属的实体
	GetEntity() Entity
}

// 保存数据接口
type Saveable interface {
	// 保存数据
	// saveData 需要保存的数据
	// isPlain 后续保存数据时,是否保持明文格式
	Save(forCache bool) (saveData interface{}, isPlain bool)
	// 加载数据(反序列化)
	Load(data interface{}) error
	// 需要保存的数据是否修改了
	IsDirty() bool
	// 设置数据修改标记
	SetDirty()
	// 重置标记
	ResetDirty()
}

// 事件接口
type EventReceiver interface {
	OnEvent(event interface{})
}

// 保存数据,并对proto进行序列化处理
func SaveWithProto(saveable Saveable, forCache bool) (interface{},error) {
	saveData,isPlain := saveable.Save(forCache)
	if !isPlain {
		// 默认对proto.Message进行序列化处理
		if protoMessage,ok := saveData.(proto.Message); ok {
			return proto.Marshal(protoMessage)
		}
	}
	return saveData,nil
}

// 加载数据,并对proto进行反序列化处理
func LoadWithProto(fromData interface{}, toProtoMessage proto.Message) error {
	if t,ok := fromData.([]byte); ok {
		err := proto.Unmarshal(t, toProtoMessage)
		if err != nil {
			logger.Error("LoadWithProto err:%v", err.Error())
			return err
		}
	}
	return nil
}
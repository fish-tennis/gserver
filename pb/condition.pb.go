// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1-devel
// 	protoc        v3.19.1
// source: condition.proto

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// 条件类型
type ConditionType int32

const (
	ConditionType_ConditionType_None          ConditionType = 0 // 解决"The first enum value must be zero in proto3."的报错
	ConditionType_ConditionType_PlayerLevelup ConditionType = 1 // 玩家升级
	ConditionType_ConditionType_Fight         ConditionType = 2 // 战斗
)

// Enum value maps for ConditionType.
var (
	ConditionType_name = map[int32]string{
		0: "ConditionType_None",
		1: "ConditionType_PlayerLevelup",
		2: "ConditionType_Fight",
	}
	ConditionType_value = map[string]int32{
		"ConditionType_None":          0,
		"ConditionType_PlayerLevelup": 1,
		"ConditionType_Fight":         2,
	}
)

func (x ConditionType) Enum() *ConditionType {
	p := new(ConditionType)
	*p = x
	return p
}

func (x ConditionType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ConditionType) Descriptor() protoreflect.EnumDescriptor {
	return file_condition_proto_enumTypes[0].Descriptor()
}

func (ConditionType) Type() protoreflect.EnumType {
	return &file_condition_proto_enumTypes[0]
}

func (x ConditionType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ConditionType.Descriptor instead.
func (ConditionType) EnumDescriptor() ([]byte, []int) {
	return file_condition_proto_rawDescGZIP(), []int{0}
}

// 玩家升级事件
type EventPlayerLevelup struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PlayerId int64 `protobuf:"varint,1,opt,name=playerId,proto3" json:"playerId,omitempty"`
	Level    int32 `protobuf:"varint,2,opt,name=level,proto3" json:"level,omitempty"`
}

func (x *EventPlayerLevelup) Reset() {
	*x = EventPlayerLevelup{}
	if protoimpl.UnsafeEnabled {
		mi := &file_condition_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventPlayerLevelup) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventPlayerLevelup) ProtoMessage() {}

func (x *EventPlayerLevelup) ProtoReflect() protoreflect.Message {
	mi := &file_condition_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventPlayerLevelup.ProtoReflect.Descriptor instead.
func (*EventPlayerLevelup) Descriptor() ([]byte, []int) {
	return file_condition_proto_rawDescGZIP(), []int{0}
}

func (x *EventPlayerLevelup) GetPlayerId() int64 {
	if x != nil {
		return x.PlayerId
	}
	return 0
}

func (x *EventPlayerLevelup) GetLevel() int32 {
	if x != nil {
		return x.Level
	}
	return 0
}

// 战斗事件
type EventFight struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PlayerId int64 `protobuf:"varint,1,opt,name=playerId,proto3" json:"playerId,omitempty"`
	IsPvp    bool  `protobuf:"varint,2,opt,name=isPvp,proto3" json:"isPvp,omitempty"`
	IsWin    bool  `protobuf:"varint,3,opt,name=isWin,proto3" json:"isWin,omitempty"`
}

func (x *EventFight) Reset() {
	*x = EventFight{}
	if protoimpl.UnsafeEnabled {
		mi := &file_condition_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventFight) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventFight) ProtoMessage() {}

func (x *EventFight) ProtoReflect() protoreflect.Message {
	mi := &file_condition_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventFight.ProtoReflect.Descriptor instead.
func (*EventFight) Descriptor() ([]byte, []int) {
	return file_condition_proto_rawDescGZIP(), []int{1}
}

func (x *EventFight) GetPlayerId() int64 {
	if x != nil {
		return x.PlayerId
	}
	return 0
}

func (x *EventFight) GetIsPvp() bool {
	if x != nil {
		return x.IsPvp
	}
	return false
}

func (x *EventFight) GetIsWin() bool {
	if x != nil {
		return x.IsWin
	}
	return false
}

var File_condition_proto protoreflect.FileDescriptor

var file_condition_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x63, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x07, 0x67, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x22, 0x46, 0x0a, 0x12, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x75, 0x70,
	0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05,
	0x6c, 0x65, 0x76, 0x65, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x6c, 0x65, 0x76,
	0x65, 0x6c, 0x22, 0x54, 0x0a, 0x0a, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x46, 0x69, 0x67, 0x68, 0x74,
	0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05,
	0x69, 0x73, 0x50, 0x76, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x69, 0x73, 0x50,
	0x76, 0x70, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x73, 0x57, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x08, 0x52, 0x05, 0x69, 0x73, 0x57, 0x69, 0x6e, 0x2a, 0x61, 0x0a, 0x0d, 0x43, 0x6f, 0x6e, 0x64,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x12, 0x16, 0x0a, 0x12, 0x43, 0x6f, 0x6e,
	0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79, 0x70, 0x65, 0x5f, 0x4e, 0x6f, 0x6e, 0x65, 0x10,
	0x00, 0x12, 0x1f, 0x0a, 0x1b, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54, 0x79,
	0x70, 0x65, 0x5f, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x75, 0x70,
	0x10, 0x01, 0x12, 0x17, 0x0a, 0x13, 0x43, 0x6f, 0x6e, 0x64, 0x69, 0x74, 0x69, 0x6f, 0x6e, 0x54,
	0x79, 0x70, 0x65, 0x5f, 0x46, 0x69, 0x67, 0x68, 0x74, 0x10, 0x02, 0x42, 0x06, 0x5a, 0x04, 0x2e,
	0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_condition_proto_rawDescOnce sync.Once
	file_condition_proto_rawDescData = file_condition_proto_rawDesc
)

func file_condition_proto_rawDescGZIP() []byte {
	file_condition_proto_rawDescOnce.Do(func() {
		file_condition_proto_rawDescData = protoimpl.X.CompressGZIP(file_condition_proto_rawDescData)
	})
	return file_condition_proto_rawDescData
}

var file_condition_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_condition_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_condition_proto_goTypes = []interface{}{
	(ConditionType)(0),         // 0: gserver.ConditionType
	(*EventPlayerLevelup)(nil), // 1: gserver.EventPlayerLevelup
	(*EventFight)(nil),         // 2: gserver.EventFight
}
var file_condition_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_condition_proto_init() }
func file_condition_proto_init() {
	if File_condition_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_condition_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventPlayerLevelup); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_condition_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventFight); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_condition_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_condition_proto_goTypes,
		DependencyIndexes: file_condition_proto_depIdxs,
		EnumInfos:         file_condition_proto_enumTypes,
		MessageInfos:      file_condition_proto_msgTypes,
	}.Build()
	File_condition_proto = out.File
	file_condition_proto_rawDesc = nil
	file_condition_proto_goTypes = nil
	file_condition_proto_depIdxs = nil
}
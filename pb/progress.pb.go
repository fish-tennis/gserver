// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.27.1-devel
// 	protoc        v3.19.1
// source: progress.proto

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

// 进度类型
type ProgressType int32

const (
	ProgressType_ProgressType_None              ProgressType = 0 // 解决"The first enum value must be zero in proto3."的报错
	ProgressType_ProgressType_PlayerLevelup     ProgressType = 1 // 玩家升级
	ProgressType_ProgressType_Fight             ProgressType = 2 // 战斗
	ProgressType_ProgressType_PlayerPropertyInc ProgressType = 3 // 玩家属性值增加(int32)
)

// Enum value maps for ProgressType.
var (
	ProgressType_name = map[int32]string{
		0: "ProgressType_None",
		1: "ProgressType_PlayerLevelup",
		2: "ProgressType_Fight",
		3: "ProgressType_PlayerPropertyInc",
	}
	ProgressType_value = map[string]int32{
		"ProgressType_None":              0,
		"ProgressType_PlayerLevelup":     1,
		"ProgressType_Fight":             2,
		"ProgressType_PlayerPropertyInc": 3,
	}
)

func (x ProgressType) Enum() *ProgressType {
	p := new(ProgressType)
	*p = x
	return p
}

func (x ProgressType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ProgressType) Descriptor() protoreflect.EnumDescriptor {
	return file_progress_proto_enumTypes[0].Descriptor()
}

func (ProgressType) Type() protoreflect.EnumType {
	return &file_progress_proto_enumTypes[0]
}

func (x ProgressType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ProgressType.Descriptor instead.
func (ProgressType) EnumDescriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{0}
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
		mi := &file_progress_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventPlayerLevelup) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventPlayerLevelup) ProtoMessage() {}

func (x *EventPlayerLevelup) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[0]
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
	return file_progress_proto_rawDescGZIP(), []int{0}
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
		mi := &file_progress_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventFight) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventFight) ProtoMessage() {}

func (x *EventFight) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[1]
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
	return file_progress_proto_rawDescGZIP(), []int{1}
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

// 玩家属性值增加(int32)
type EventPlayerPropertyInc struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PlayerId      int64  `protobuf:"varint,1,opt,name=playerId,proto3" json:"playerId,omitempty"`
	PropertyName  string `protobuf:"bytes,2,opt,name=propertyName,proto3" json:"propertyName,omitempty"`
	PropertyValue int32  `protobuf:"varint,3,opt,name=propertyValue,proto3" json:"propertyValue,omitempty"`
}

func (x *EventPlayerPropertyInc) Reset() {
	*x = EventPlayerPropertyInc{}
	if protoimpl.UnsafeEnabled {
		mi := &file_progress_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *EventPlayerPropertyInc) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*EventPlayerPropertyInc) ProtoMessage() {}

func (x *EventPlayerPropertyInc) ProtoReflect() protoreflect.Message {
	mi := &file_progress_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use EventPlayerPropertyInc.ProtoReflect.Descriptor instead.
func (*EventPlayerPropertyInc) Descriptor() ([]byte, []int) {
	return file_progress_proto_rawDescGZIP(), []int{2}
}

func (x *EventPlayerPropertyInc) GetPlayerId() int64 {
	if x != nil {
		return x.PlayerId
	}
	return 0
}

func (x *EventPlayerPropertyInc) GetPropertyName() string {
	if x != nil {
		return x.PropertyName
	}
	return ""
}

func (x *EventPlayerPropertyInc) GetPropertyValue() int32 {
	if x != nil {
		return x.PropertyValue
	}
	return 0
}

var File_progress_proto protoreflect.FileDescriptor

var file_progress_proto_rawDesc = []byte{
	0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x07, 0x67, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x22, 0x46, 0x0a, 0x12, 0x45, 0x76, 0x65,
	0x6e, 0x74, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x75, 0x70, 0x12,
	0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x6c,
	0x65, 0x76, 0x65, 0x6c, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x05, 0x6c, 0x65, 0x76, 0x65,
	0x6c, 0x22, 0x54, 0x0a, 0x0a, 0x45, 0x76, 0x65, 0x6e, 0x74, 0x46, 0x69, 0x67, 0x68, 0x74, 0x12,
	0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x03, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x12, 0x14, 0x0a, 0x05, 0x69,
	0x73, 0x50, 0x76, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x69, 0x73, 0x50, 0x76,
	0x70, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x73, 0x57, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x05, 0x69, 0x73, 0x57, 0x69, 0x6e, 0x22, 0x7e, 0x0a, 0x16, 0x45, 0x76, 0x65, 0x6e, 0x74,
	0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x50, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x49, 0x6e,
	0x63, 0x12, 0x1a, 0x0a, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x03, 0x52, 0x08, 0x70, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x49, 0x64, 0x12, 0x22, 0x0a,
	0x0c, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0c, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x4e, 0x61, 0x6d,
	0x65, 0x12, 0x24, 0x0a, 0x0d, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x05, 0x52, 0x0d, 0x70, 0x72, 0x6f, 0x70, 0x65, 0x72,
	0x74, 0x79, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x2a, 0x81, 0x01, 0x0a, 0x0c, 0x50, 0x72, 0x6f, 0x67,
	0x72, 0x65, 0x73, 0x73, 0x54, 0x79, 0x70, 0x65, 0x12, 0x15, 0x0a, 0x11, 0x50, 0x72, 0x6f, 0x67,
	0x72, 0x65, 0x73, 0x73, 0x54, 0x79, 0x70, 0x65, 0x5f, 0x4e, 0x6f, 0x6e, 0x65, 0x10, 0x00, 0x12,
	0x1e, 0x0a, 0x1a, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x54, 0x79, 0x70, 0x65, 0x5f,
	0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x75, 0x70, 0x10, 0x01, 0x12,
	0x16, 0x0a, 0x12, 0x50, 0x72, 0x6f, 0x67, 0x72, 0x65, 0x73, 0x73, 0x54, 0x79, 0x70, 0x65, 0x5f,
	0x46, 0x69, 0x67, 0x68, 0x74, 0x10, 0x02, 0x12, 0x22, 0x0a, 0x1e, 0x50, 0x72, 0x6f, 0x67, 0x72,
	0x65, 0x73, 0x73, 0x54, 0x79, 0x70, 0x65, 0x5f, 0x50, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x50, 0x72,
	0x6f, 0x70, 0x65, 0x72, 0x74, 0x79, 0x49, 0x6e, 0x63, 0x10, 0x03, 0x42, 0x06, 0x5a, 0x04, 0x2e,
	0x2f, 0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_progress_proto_rawDescOnce sync.Once
	file_progress_proto_rawDescData = file_progress_proto_rawDesc
)

func file_progress_proto_rawDescGZIP() []byte {
	file_progress_proto_rawDescOnce.Do(func() {
		file_progress_proto_rawDescData = protoimpl.X.CompressGZIP(file_progress_proto_rawDescData)
	})
	return file_progress_proto_rawDescData
}

var file_progress_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_progress_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_progress_proto_goTypes = []interface{}{
	(ProgressType)(0),              // 0: gserver.ProgressType
	(*EventPlayerLevelup)(nil),     // 1: gserver.EventPlayerLevelup
	(*EventFight)(nil),             // 2: gserver.EventFight
	(*EventPlayerPropertyInc)(nil), // 3: gserver.EventPlayerPropertyInc
}
var file_progress_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_progress_proto_init() }
func file_progress_proto_init() {
	if File_progress_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_progress_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
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
		file_progress_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
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
		file_progress_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*EventPlayerPropertyInc); i {
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
			RawDescriptor: file_progress_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_progress_proto_goTypes,
		DependencyIndexes: file_progress_proto_depIdxs,
		EnumInfos:         file_progress_proto_enumTypes,
		MessageInfos:      file_progress_proto_msgTypes,
	}.Build()
	File_progress_proto = out.File
	file_progress_proto_rawDesc = nil
	file_progress_proto_goTypes = nil
	file_progress_proto_depIdxs = nil
}
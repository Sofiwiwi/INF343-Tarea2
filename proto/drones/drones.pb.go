// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.14.0
// source: proto/drones.proto

package drones

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	timestamppb "google.golang.org/protobuf/types/known/timestamppb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// Solicitud de asignación de emergencia de Asignación a Drones
type AssignEmergencyRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	EmergencyId    int64                  `protobuf:"varint,1,opt,name=emergency_id,json=emergencyId,proto3" json:"emergency_id,omitempty"`
	EmergencyName  string                 `protobuf:"bytes,2,opt,name=emergency_name,json=emergencyName,proto3" json:"emergency_name,omitempty"`
	Latitude       float64                `protobuf:"fixed64,3,opt,name=latitude,proto3" json:"latitude,omitempty"`
	Longitude      float64                `protobuf:"fixed64,4,opt,name=longitude,proto3" json:"longitude,omitempty"`
	Magnitude      int32                  `protobuf:"varint,5,opt,name=magnitude,proto3" json:"magnitude,omitempty"`
	DronId         string                 `protobuf:"bytes,6,opt,name=dron_id,json=dronId,proto3" json:"dron_id,omitempty"`
	AssignmentTime *timestamppb.Timestamp `protobuf:"bytes,7,opt,name=assignment_time,json=assignmentTime,proto3" json:"assignment_time,omitempty"`
}

func (x *AssignEmergencyRequest) Reset() {
	*x = AssignEmergencyRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_drones_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AssignEmergencyRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AssignEmergencyRequest) ProtoMessage() {}

func (x *AssignEmergencyRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_drones_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AssignEmergencyRequest.ProtoReflect.Descriptor instead.
func (*AssignEmergencyRequest) Descriptor() ([]byte, []int) {
	return file_proto_drones_proto_rawDescGZIP(), []int{0}
}

func (x *AssignEmergencyRequest) GetEmergencyId() int64 {
	if x != nil {
		return x.EmergencyId
	}
	return 0
}

func (x *AssignEmergencyRequest) GetEmergencyName() string {
	if x != nil {
		return x.EmergencyName
	}
	return ""
}

func (x *AssignEmergencyRequest) GetLatitude() float64 {
	if x != nil {
		return x.Latitude
	}
	return 0
}

func (x *AssignEmergencyRequest) GetLongitude() float64 {
	if x != nil {
		return x.Longitude
	}
	return 0
}

func (x *AssignEmergencyRequest) GetMagnitude() int32 {
	if x != nil {
		return x.Magnitude
	}
	return 0
}

func (x *AssignEmergencyRequest) GetDronId() string {
	if x != nil {
		return x.DronId
	}
	return ""
}

func (x *AssignEmergencyRequest) GetAssignmentTime() *timestamppb.Timestamp {
	if x != nil {
		return x.AssignmentTime
	}
	return nil
}

// Respuesta de asignación de emergencia de Drones a Asignación
type AssignEmergencyResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Success bool   `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *AssignEmergencyResponse) Reset() {
	*x = AssignEmergencyResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_drones_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *AssignEmergencyResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*AssignEmergencyResponse) ProtoMessage() {}

func (x *AssignEmergencyResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_drones_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use AssignEmergencyResponse.ProtoReflect.Descriptor instead.
func (*AssignEmergencyResponse) Descriptor() ([]byte, []int) {
	return file_proto_drones_proto_rawDescGZIP(), []int{1}
}

func (x *AssignEmergencyResponse) GetSuccess() bool {
	if x != nil {
		return x.Success
	}
	return false
}

func (x *AssignEmergencyResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_proto_drones_proto protoreflect.FileDescriptor

var file_proto_drones_proto_rawDesc = []byte{
	0x0a, 0x12, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x64, 0x72, 0x6f, 0x6e, 0x65, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x64, 0x72, 0x6f, 0x6e, 0x65, 0x73, 0x1a, 0x1f, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x74, 0x69,
	0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x98, 0x02,
	0x0a, 0x16, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x45, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63,
	0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x65, 0x6d, 0x65, 0x72,
	0x67, 0x65, 0x6e, 0x63, 0x79, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b,
	0x65, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x49, 0x64, 0x12, 0x25, 0x0a, 0x0e, 0x65,
	0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x0d, 0x65, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x4e, 0x61,
	0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x6c, 0x61, 0x74, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18, 0x03,
	0x20, 0x01, 0x28, 0x01, 0x52, 0x08, 0x6c, 0x61, 0x74, 0x69, 0x74, 0x75, 0x64, 0x65, 0x12, 0x1c,
	0x0a, 0x09, 0x6c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x01, 0x52, 0x09, 0x6c, 0x6f, 0x6e, 0x67, 0x69, 0x74, 0x75, 0x64, 0x65, 0x12, 0x1c, 0x0a, 0x09,
	0x6d, 0x61, 0x67, 0x6e, 0x69, 0x74, 0x75, 0x64, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x05, 0x52,
	0x09, 0x6d, 0x61, 0x67, 0x6e, 0x69, 0x74, 0x75, 0x64, 0x65, 0x12, 0x17, 0x0a, 0x07, 0x64, 0x72,
	0x6f, 0x6e, 0x5f, 0x69, 0x64, 0x18, 0x06, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x64, 0x72, 0x6f,
	0x6e, 0x49, 0x64, 0x12, 0x43, 0x0a, 0x0f, 0x61, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x6d, 0x65, 0x6e,
	0x74, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x07, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x54,
	0x69, 0x6d, 0x65, 0x73, 0x74, 0x61, 0x6d, 0x70, 0x52, 0x0e, 0x61, 0x73, 0x73, 0x69, 0x67, 0x6e,
	0x6d, 0x65, 0x6e, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x22, 0x4d, 0x0a, 0x17, 0x41, 0x73, 0x73, 0x69,
	0x67, 0x6e, 0x45, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x12, 0x18, 0x0a,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x32, 0x63, 0x0a, 0x0d, 0x44, 0x72, 0x6f, 0x6e, 0x65,
	0x73, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x52, 0x0a, 0x0f, 0x41, 0x73, 0x73, 0x69,
	0x67, 0x6e, 0x45, 0x6d, 0x65, 0x72, 0x67, 0x65, 0x6e, 0x63, 0x79, 0x12, 0x1e, 0x2e, 0x64, 0x72,
	0x6f, 0x6e, 0x65, 0x73, 0x2e, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x45, 0x6d, 0x65, 0x72, 0x67,
	0x65, 0x6e, 0x63, 0x79, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1f, 0x2e, 0x64, 0x72,
	0x6f, 0x6e, 0x65, 0x73, 0x2e, 0x41, 0x73, 0x73, 0x69, 0x67, 0x6e, 0x45, 0x6d, 0x65, 0x72, 0x67,
	0x65, 0x6e, 0x63, 0x79, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x42, 0x1c, 0x5a, 0x1a,
	0x49, 0x4e, 0x46, 0x33, 0x34, 0x33, 0x2d, 0x54, 0x61, 0x72, 0x65, 0x61, 0x32, 0x2f, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x2f, 0x64, 0x72, 0x6f, 0x6e, 0x65, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_proto_drones_proto_rawDescOnce sync.Once
	file_proto_drones_proto_rawDescData = file_proto_drones_proto_rawDesc
)

func file_proto_drones_proto_rawDescGZIP() []byte {
	file_proto_drones_proto_rawDescOnce.Do(func() {
		file_proto_drones_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_drones_proto_rawDescData)
	})
	return file_proto_drones_proto_rawDescData
}

var file_proto_drones_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_proto_drones_proto_goTypes = []interface{}{
	(*AssignEmergencyRequest)(nil),  // 0: drones.AssignEmergencyRequest
	(*AssignEmergencyResponse)(nil), // 1: drones.AssignEmergencyResponse
	(*timestamppb.Timestamp)(nil),   // 2: google.protobuf.Timestamp
}
var file_proto_drones_proto_depIdxs = []int32{
	2, // 0: drones.AssignEmergencyRequest.assignment_time:type_name -> google.protobuf.Timestamp
	0, // 1: drones.DronesService.AssignEmergency:input_type -> drones.AssignEmergencyRequest
	1, // 2: drones.DronesService.AssignEmergency:output_type -> drones.AssignEmergencyResponse
	2, // [2:3] is the sub-list for method output_type
	1, // [1:2] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_proto_drones_proto_init() }
func file_proto_drones_proto_init() {
	if File_proto_drones_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_drones_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AssignEmergencyRequest); i {
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
		file_proto_drones_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*AssignEmergencyResponse); i {
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
			RawDescriptor: file_proto_drones_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_drones_proto_goTypes,
		DependencyIndexes: file_proto_drones_proto_depIdxs,
		MessageInfos:      file_proto_drones_proto_msgTypes,
	}.Build()
	File_proto_drones_proto = out.File
	file_proto_drones_proto_rawDesc = nil
	file_proto_drones_proto_goTypes = nil
	file_proto_drones_proto_depIdxs = nil
}

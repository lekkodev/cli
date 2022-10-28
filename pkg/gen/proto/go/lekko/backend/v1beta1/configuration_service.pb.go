// Copyright 2022 Lekko Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.0
// 	protoc        (unknown)
// source: lekko/backend/v1beta1/configuration_service.proto

package backendv1beta1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	anypb "google.golang.org/protobuf/types/known/anypb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type RepositoryKey struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	OwnerName string `protobuf:"bytes,1,opt,name=owner_name,json=ownerName,proto3" json:"owner_name,omitempty"`
	RepoName  string `protobuf:"bytes,2,opt,name=repo_name,json=repoName,proto3" json:"repo_name,omitempty"`
}

func (x *RepositoryKey) Reset() {
	*x = RepositoryKey{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RepositoryKey) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RepositoryKey) ProtoMessage() {}

func (x *RepositoryKey) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RepositoryKey.ProtoReflect.Descriptor instead.
func (*RepositoryKey) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{0}
}

func (x *RepositoryKey) GetOwnerName() string {
	if x != nil {
		return x.OwnerName
	}
	return ""
}

func (x *RepositoryKey) GetRepoName() string {
	if x != nil {
		return x.RepoName
	}
	return ""
}

type GetBoolValueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key       string            `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Context   map[string]*Value `protobuf:"bytes,2,rep,name=context,proto3" json:"context,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Namespace string            `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	RepoKey   *RepositoryKey    `protobuf:"bytes,4,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
}

func (x *GetBoolValueRequest) Reset() {
	*x = GetBoolValueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBoolValueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBoolValueRequest) ProtoMessage() {}

func (x *GetBoolValueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBoolValueRequest.ProtoReflect.Descriptor instead.
func (*GetBoolValueRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{1}
}

func (x *GetBoolValueRequest) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *GetBoolValueRequest) GetContext() map[string]*Value {
	if x != nil {
		return x.Context
	}
	return nil
}

func (x *GetBoolValueRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *GetBoolValueRequest) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

type GetBoolValueResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value bool `protobuf:"varint,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *GetBoolValueResponse) Reset() {
	*x = GetBoolValueResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetBoolValueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetBoolValueResponse) ProtoMessage() {}

func (x *GetBoolValueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetBoolValueResponse.ProtoReflect.Descriptor instead.
func (*GetBoolValueResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{2}
}

func (x *GetBoolValueResponse) GetValue() bool {
	if x != nil {
		return x.Value
	}
	return false
}

type GetProtoValueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key       string            `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Context   map[string]*Value `protobuf:"bytes,2,rep,name=context,proto3" json:"context,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Namespace string            `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	RepoKey   *RepositoryKey    `protobuf:"bytes,4,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
}

func (x *GetProtoValueRequest) Reset() {
	*x = GetProtoValueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProtoValueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProtoValueRequest) ProtoMessage() {}

func (x *GetProtoValueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProtoValueRequest.ProtoReflect.Descriptor instead.
func (*GetProtoValueRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{3}
}

func (x *GetProtoValueRequest) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *GetProtoValueRequest) GetContext() map[string]*Value {
	if x != nil {
		return x.Context
	}
	return nil
}

func (x *GetProtoValueRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *GetProtoValueRequest) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

type GetProtoValueResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value *anypb.Any `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *GetProtoValueResponse) Reset() {
	*x = GetProtoValueResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetProtoValueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetProtoValueResponse) ProtoMessage() {}

func (x *GetProtoValueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetProtoValueResponse.ProtoReflect.Descriptor instead.
func (*GetProtoValueResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{4}
}

func (x *GetProtoValueResponse) GetValue() *anypb.Any {
	if x != nil {
		return x.Value
	}
	return nil
}

type GetJSONValueRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key       string            `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Context   map[string]*Value `protobuf:"bytes,2,rep,name=context,proto3" json:"context,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"bytes,2,opt,name=value,proto3"`
	Namespace string            `protobuf:"bytes,3,opt,name=namespace,proto3" json:"namespace,omitempty"`
	RepoKey   *RepositoryKey    `protobuf:"bytes,4,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
}

func (x *GetJSONValueRequest) Reset() {
	*x = GetJSONValueRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetJSONValueRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetJSONValueRequest) ProtoMessage() {}

func (x *GetJSONValueRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetJSONValueRequest.ProtoReflect.Descriptor instead.
func (*GetJSONValueRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{5}
}

func (x *GetJSONValueRequest) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *GetJSONValueRequest) GetContext() map[string]*Value {
	if x != nil {
		return x.Context
	}
	return nil
}

func (x *GetJSONValueRequest) GetNamespace() string {
	if x != nil {
		return x.Namespace
	}
	return ""
}

func (x *GetJSONValueRequest) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

type GetJSONValueResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Value []byte `protobuf:"bytes,1,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *GetJSONValueResponse) Reset() {
	*x = GetJSONValueResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetJSONValueResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetJSONValueResponse) ProtoMessage() {}

func (x *GetJSONValueResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetJSONValueResponse.ProtoReflect.Descriptor instead.
func (*GetJSONValueResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{6}
}

func (x *GetJSONValueResponse) GetValue() []byte {
	if x != nil {
		return x.Value
	}
	return nil
}

type Value struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Kind:
	//	*Value_BoolValue
	//	*Value_IntValue
	//	*Value_DoubleValue
	//	*Value_StringValue
	Kind isValue_Kind `protobuf_oneof:"kind"`
}

func (x *Value) Reset() {
	*x = Value{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Value) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Value) ProtoMessage() {}

func (x *Value) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Value.ProtoReflect.Descriptor instead.
func (*Value) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP(), []int{7}
}

func (m *Value) GetKind() isValue_Kind {
	if m != nil {
		return m.Kind
	}
	return nil
}

func (x *Value) GetBoolValue() bool {
	if x, ok := x.GetKind().(*Value_BoolValue); ok {
		return x.BoolValue
	}
	return false
}

func (x *Value) GetIntValue() int64 {
	if x, ok := x.GetKind().(*Value_IntValue); ok {
		return x.IntValue
	}
	return 0
}

func (x *Value) GetDoubleValue() float64 {
	if x, ok := x.GetKind().(*Value_DoubleValue); ok {
		return x.DoubleValue
	}
	return 0
}

func (x *Value) GetStringValue() string {
	if x, ok := x.GetKind().(*Value_StringValue); ok {
		return x.StringValue
	}
	return ""
}

type isValue_Kind interface {
	isValue_Kind()
}

type Value_BoolValue struct {
	BoolValue bool `protobuf:"varint,1,opt,name=bool_value,json=boolValue,proto3,oneof"`
}

type Value_IntValue struct {
	IntValue int64 `protobuf:"varint,2,opt,name=int_value,json=intValue,proto3,oneof"`
}

type Value_DoubleValue struct {
	DoubleValue float64 `protobuf:"fixed64,3,opt,name=double_value,json=doubleValue,proto3,oneof"`
}

type Value_StringValue struct {
	StringValue string `protobuf:"bytes,4,opt,name=string_value,json=stringValue,proto3,oneof"`
}

func (*Value_BoolValue) isValue_Kind() {}

func (*Value_IntValue) isValue_Kind() {}

func (*Value_DoubleValue) isValue_Kind() {}

func (*Value_StringValue) isValue_Kind() {}

var File_lekko_backend_v1beta1_configuration_service_proto protoreflect.FileDescriptor

var file_lekko_backend_v1beta1_configuration_service_proto_rawDesc = []byte{
	0x0a, 0x31, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2f,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x15, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65,
	0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x1a, 0x19, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x61, 0x6e, 0x79, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4b, 0x0a, 0x0d, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x12, 0x1d, 0x0a, 0x0a, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x5f,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6f, 0x77, 0x6e, 0x65,
	0x72, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1b, 0x0a, 0x09, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x4e, 0x61,
	0x6d, 0x65, 0x22, 0xb3, 0x02, 0x0a, 0x13, 0x47, 0x65, 0x74, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65,
	0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x51, 0x0a, 0x07,
	0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x37, 0x2e,
	0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78,
	0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x12,
	0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x3f, 0x0a,
	0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f,
	0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70, 0x6f, 0x4b, 0x65, 0x79, 0x1a, 0x58,
	0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10,
	0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79,
	0x12, 0x32, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x1c, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05, 0x76,
	0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x2c, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x42,
	0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0xb5, 0x02, 0x0a, 0x14, 0x47, 0x65, 0x74, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x52, 0x0a, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x38, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65,
	0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e,
	0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x63, 0x6f,
	0x6e, 0x74, 0x65, 0x78, 0x74, 0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61,
	0x63, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x12, 0x3f, 0x0a, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18,
	0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70,
	0x6f, 0x4b, 0x65, 0x79, 0x1a, 0x58, 0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x32, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1c, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x43,
	0x0a, 0x15, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x2a, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x41, 0x6e, 0x79, 0x52, 0x05, 0x76, 0x61,
	0x6c, 0x75, 0x65, 0x22, 0xb3, 0x02, 0x0a, 0x13, 0x47, 0x65, 0x74, 0x4a, 0x53, 0x4f, 0x4e, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x6b,
	0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x51, 0x0a,
	0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x37,
	0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x4a, 0x53, 0x4f, 0x4e, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x2e, 0x43, 0x6f, 0x6e, 0x74, 0x65,
	0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x52, 0x07, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74,
	0x12, 0x1c, 0x0a, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x18, 0x03, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x09, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x3f,
	0x0a, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70, 0x6f, 0x4b, 0x65, 0x79, 0x1a,
	0x58, 0x0a, 0x0c, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12,
	0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65,
	0x79, 0x12, 0x32, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x1c, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x05,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x2c, 0x0a, 0x14, 0x47, 0x65, 0x74,
	0x4a, 0x53, 0x4f, 0x4e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c,
	0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x99, 0x01, 0x0a, 0x05, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x12, 0x1f, 0x0a, 0x0a, 0x62, 0x6f, 0x6f, 0x6c, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x08, 0x48, 0x00, 0x52, 0x09, 0x62, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c,
	0x75, 0x65, 0x12, 0x1d, 0x0a, 0x09, 0x69, 0x6e, 0x74, 0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x48, 0x00, 0x52, 0x08, 0x69, 0x6e, 0x74, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x12, 0x23, 0x0a, 0x0c, 0x64, 0x6f, 0x75, 0x62, 0x6c, 0x65, 0x5f, 0x76, 0x61, 0x6c, 0x75,
	0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x01, 0x48, 0x00, 0x52, 0x0b, 0x64, 0x6f, 0x75, 0x62, 0x6c,
	0x65, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x23, 0x0a, 0x0c, 0x73, 0x74, 0x72, 0x69, 0x6e, 0x67,
	0x5f, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x48, 0x00, 0x52, 0x0b,
	0x73, 0x74, 0x72, 0x69, 0x6e, 0x67, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x42, 0x06, 0x0a, 0x04, 0x6b,
	0x69, 0x6e, 0x64, 0x32, 0xda, 0x02, 0x0a, 0x14, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72,
	0x61, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x69, 0x0a, 0x0c,
	0x47, 0x65, 0x74, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x2a, 0x2e, 0x6c,
	0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75,
	0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2b, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f,
	0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x47, 0x65, 0x74, 0x42, 0x6f, 0x6f, 0x6c, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x6c, 0x0a, 0x0d, 0x47, 0x65, 0x74, 0x50, 0x72,
	0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x2b, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f,
	0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x47, 0x65, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x2c, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65,
	0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x69, 0x0a, 0x0c, 0x47, 0x65, 0x74, 0x4a, 0x53, 0x4f, 0x4e,
	0x56, 0x61, 0x6c, 0x75, 0x65, 0x12, 0x2a, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65,
	0x74, 0x4a, 0x53, 0x4f, 0x4e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x2b, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e,
	0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x4a, 0x53, 0x4f,
	0x4e, 0x56, 0x61, 0x6c, 0x75, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x42, 0xfb, 0x01, 0x0a, 0x19, 0x63, 0x6f, 0x6d, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62,
	0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42, 0x19,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x53, 0x65, 0x72,
	0x76, 0x69, 0x63, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x4d, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x64, 0x65, 0x76,
	0x2f, 0x63, 0x6c, 0x69, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x2f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x62, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x3b, 0x62, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0xa2, 0x02, 0x03, 0x4c, 0x42, 0x58,
	0xaa, 0x02, 0x15, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x2e, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca, 0x02, 0x15, 0x4c, 0x65, 0x6b, 0x6b, 0x6f,
	0x5c, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0xe2, 0x02, 0x21, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x5c, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64,
	0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61,
	0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x17, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x3a, 0x3a, 0x42, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_lekko_backend_v1beta1_configuration_service_proto_rawDescOnce sync.Once
	file_lekko_backend_v1beta1_configuration_service_proto_rawDescData = file_lekko_backend_v1beta1_configuration_service_proto_rawDesc
)

func file_lekko_backend_v1beta1_configuration_service_proto_rawDescGZIP() []byte {
	file_lekko_backend_v1beta1_configuration_service_proto_rawDescOnce.Do(func() {
		file_lekko_backend_v1beta1_configuration_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_lekko_backend_v1beta1_configuration_service_proto_rawDescData)
	})
	return file_lekko_backend_v1beta1_configuration_service_proto_rawDescData
}

var file_lekko_backend_v1beta1_configuration_service_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_lekko_backend_v1beta1_configuration_service_proto_goTypes = []interface{}{
	(*RepositoryKey)(nil),         // 0: lekko.backend.v1beta1.RepositoryKey
	(*GetBoolValueRequest)(nil),   // 1: lekko.backend.v1beta1.GetBoolValueRequest
	(*GetBoolValueResponse)(nil),  // 2: lekko.backend.v1beta1.GetBoolValueResponse
	(*GetProtoValueRequest)(nil),  // 3: lekko.backend.v1beta1.GetProtoValueRequest
	(*GetProtoValueResponse)(nil), // 4: lekko.backend.v1beta1.GetProtoValueResponse
	(*GetJSONValueRequest)(nil),   // 5: lekko.backend.v1beta1.GetJSONValueRequest
	(*GetJSONValueResponse)(nil),  // 6: lekko.backend.v1beta1.GetJSONValueResponse
	(*Value)(nil),                 // 7: lekko.backend.v1beta1.Value
	nil,                           // 8: lekko.backend.v1beta1.GetBoolValueRequest.ContextEntry
	nil,                           // 9: lekko.backend.v1beta1.GetProtoValueRequest.ContextEntry
	nil,                           // 10: lekko.backend.v1beta1.GetJSONValueRequest.ContextEntry
	(*anypb.Any)(nil),             // 11: google.protobuf.Any
}
var file_lekko_backend_v1beta1_configuration_service_proto_depIdxs = []int32{
	8,  // 0: lekko.backend.v1beta1.GetBoolValueRequest.context:type_name -> lekko.backend.v1beta1.GetBoolValueRequest.ContextEntry
	0,  // 1: lekko.backend.v1beta1.GetBoolValueRequest.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	9,  // 2: lekko.backend.v1beta1.GetProtoValueRequest.context:type_name -> lekko.backend.v1beta1.GetProtoValueRequest.ContextEntry
	0,  // 3: lekko.backend.v1beta1.GetProtoValueRequest.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	11, // 4: lekko.backend.v1beta1.GetProtoValueResponse.value:type_name -> google.protobuf.Any
	10, // 5: lekko.backend.v1beta1.GetJSONValueRequest.context:type_name -> lekko.backend.v1beta1.GetJSONValueRequest.ContextEntry
	0,  // 6: lekko.backend.v1beta1.GetJSONValueRequest.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	7,  // 7: lekko.backend.v1beta1.GetBoolValueRequest.ContextEntry.value:type_name -> lekko.backend.v1beta1.Value
	7,  // 8: lekko.backend.v1beta1.GetProtoValueRequest.ContextEntry.value:type_name -> lekko.backend.v1beta1.Value
	7,  // 9: lekko.backend.v1beta1.GetJSONValueRequest.ContextEntry.value:type_name -> lekko.backend.v1beta1.Value
	1,  // 10: lekko.backend.v1beta1.ConfigurationService.GetBoolValue:input_type -> lekko.backend.v1beta1.GetBoolValueRequest
	3,  // 11: lekko.backend.v1beta1.ConfigurationService.GetProtoValue:input_type -> lekko.backend.v1beta1.GetProtoValueRequest
	5,  // 12: lekko.backend.v1beta1.ConfigurationService.GetJSONValue:input_type -> lekko.backend.v1beta1.GetJSONValueRequest
	2,  // 13: lekko.backend.v1beta1.ConfigurationService.GetBoolValue:output_type -> lekko.backend.v1beta1.GetBoolValueResponse
	4,  // 14: lekko.backend.v1beta1.ConfigurationService.GetProtoValue:output_type -> lekko.backend.v1beta1.GetProtoValueResponse
	6,  // 15: lekko.backend.v1beta1.ConfigurationService.GetJSONValue:output_type -> lekko.backend.v1beta1.GetJSONValueResponse
	13, // [13:16] is the sub-list for method output_type
	10, // [10:13] is the sub-list for method input_type
	10, // [10:10] is the sub-list for extension type_name
	10, // [10:10] is the sub-list for extension extendee
	0,  // [0:10] is the sub-list for field type_name
}

func init() { file_lekko_backend_v1beta1_configuration_service_proto_init() }
func file_lekko_backend_v1beta1_configuration_service_proto_init() {
	if File_lekko_backend_v1beta1_configuration_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RepositoryKey); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBoolValueRequest); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetBoolValueResponse); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProtoValueRequest); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetProtoValueResponse); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetJSONValueRequest); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetJSONValueResponse); i {
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
		file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Value); i {
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
	file_lekko_backend_v1beta1_configuration_service_proto_msgTypes[7].OneofWrappers = []interface{}{
		(*Value_BoolValue)(nil),
		(*Value_IntValue)(nil),
		(*Value_DoubleValue)(nil),
		(*Value_StringValue)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_lekko_backend_v1beta1_configuration_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_lekko_backend_v1beta1_configuration_service_proto_goTypes,
		DependencyIndexes: file_lekko_backend_v1beta1_configuration_service_proto_depIdxs,
		MessageInfos:      file_lekko_backend_v1beta1_configuration_service_proto_msgTypes,
	}.Build()
	File_lekko_backend_v1beta1_configuration_service_proto = out.File
	file_lekko_backend_v1beta1_configuration_service_proto_rawDesc = nil
	file_lekko_backend_v1beta1_configuration_service_proto_goTypes = nil
	file_lekko_backend_v1beta1_configuration_service_proto_depIdxs = nil
}

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
// source: lekko/backend/v1beta1/distribution_service.proto

package backendv1beta1

import (
	v1beta1 "github.com/lekkodev/cli/pkg/gen/proto/go/lekko/feature/v1beta1"
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

type GetRepositoryVersionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RepoKey *RepositoryKey `protobuf:"bytes,1,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
}

func (x *GetRepositoryVersionRequest) Reset() {
	*x = GetRepositoryVersionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetRepositoryVersionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryVersionRequest) ProtoMessage() {}

func (x *GetRepositoryVersionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryVersionRequest.ProtoReflect.Descriptor instead.
func (*GetRepositoryVersionRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{0}
}

func (x *GetRepositoryVersionRequest) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

type GetRepositoryVersionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CommitSha string `protobuf:"bytes,1,opt,name=commit_sha,json=commitSha,proto3" json:"commit_sha,omitempty"`
}

func (x *GetRepositoryVersionResponse) Reset() {
	*x = GetRepositoryVersionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetRepositoryVersionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryVersionResponse) ProtoMessage() {}

func (x *GetRepositoryVersionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryVersionResponse.ProtoReflect.Descriptor instead.
func (*GetRepositoryVersionResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{1}
}

func (x *GetRepositoryVersionResponse) GetCommitSha() string {
	if x != nil {
		return x.CommitSha
	}
	return ""
}

type GetRepositoryContentsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RepoKey *RepositoryKey `protobuf:"bytes,1,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
	// optional namespace_name to filter responses by
	NamespaceName string `protobuf:"bytes,2,opt,name=namespace_name,json=namespaceName,proto3" json:"namespace_name,omitempty"`
	// optional feature_name to filter responses by
	FeatureName string `protobuf:"bytes,3,opt,name=feature_name,json=featureName,proto3" json:"feature_name,omitempty"`
}

func (x *GetRepositoryContentsRequest) Reset() {
	*x = GetRepositoryContentsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetRepositoryContentsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryContentsRequest) ProtoMessage() {}

func (x *GetRepositoryContentsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryContentsRequest.ProtoReflect.Descriptor instead.
func (*GetRepositoryContentsRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{2}
}

func (x *GetRepositoryContentsRequest) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

func (x *GetRepositoryContentsRequest) GetNamespaceName() string {
	if x != nil {
		return x.NamespaceName
	}
	return ""
}

func (x *GetRepositoryContentsRequest) GetFeatureName() string {
	if x != nil {
		return x.FeatureName
	}
	return ""
}

type GetRepositoryContentsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CommitSha  string       `protobuf:"bytes,1,opt,name=commit_sha,json=commitSha,proto3" json:"commit_sha,omitempty"`
	Namespaces []*Namespace `protobuf:"bytes,2,rep,name=namespaces,proto3" json:"namespaces,omitempty"`
}

func (x *GetRepositoryContentsResponse) Reset() {
	*x = GetRepositoryContentsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetRepositoryContentsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetRepositoryContentsResponse) ProtoMessage() {}

func (x *GetRepositoryContentsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetRepositoryContentsResponse.ProtoReflect.Descriptor instead.
func (*GetRepositoryContentsResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{3}
}

func (x *GetRepositoryContentsResponse) GetCommitSha() string {
	if x != nil {
		return x.CommitSha
	}
	return ""
}

func (x *GetRepositoryContentsResponse) GetNamespaces() []*Namespace {
	if x != nil {
		return x.Namespaces
	}
	return nil
}

type Namespace struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name     string     `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Features []*Feature `protobuf:"bytes,2,rep,name=features,proto3" json:"features,omitempty"`
}

func (x *Namespace) Reset() {
	*x = Namespace{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Namespace) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Namespace) ProtoMessage() {}

func (x *Namespace) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Namespace.ProtoReflect.Descriptor instead.
func (*Namespace) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{4}
}

func (x *Namespace) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Namespace) GetFeatures() []*Feature {
	if x != nil {
		return x.Features
	}
	return nil
}

type Feature struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	// The sha of the protobuf binary according to git.
	Sha     string           `protobuf:"bytes,2,opt,name=sha,proto3" json:"sha,omitempty"`
	Feature *v1beta1.Feature `protobuf:"bytes,3,opt,name=feature,proto3" json:"feature,omitempty"`
}

func (x *Feature) Reset() {
	*x = Feature{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Feature) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Feature) ProtoMessage() {}

func (x *Feature) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Feature.ProtoReflect.Descriptor instead.
func (*Feature) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{5}
}

func (x *Feature) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Feature) GetSha() string {
	if x != nil {
		return x.Sha
	}
	return ""
}

func (x *Feature) GetFeature() *v1beta1.Feature {
	if x != nil {
		return x.Feature
	}
	return nil
}

type ContextKey struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Key  string `protobuf:"bytes,1,opt,name=key,proto3" json:"key,omitempty"`
	Type string `protobuf:"bytes,2,opt,name=type,proto3" json:"type,omitempty"`
}

func (x *ContextKey) Reset() {
	*x = ContextKey{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ContextKey) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ContextKey) ProtoMessage() {}

func (x *ContextKey) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ContextKey.ProtoReflect.Descriptor instead.
func (*ContextKey) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{6}
}

func (x *ContextKey) GetKey() string {
	if x != nil {
		return x.Key
	}
	return ""
}

func (x *ContextKey) GetType() string {
	if x != nil {
		return x.Type
	}
	return ""
}

type FlagEvaluationEvent struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RepoKey       *RepositoryKey `protobuf:"bytes,1,opt,name=repo_key,json=repoKey,proto3" json:"repo_key,omitempty"`
	CommitSha     string         `protobuf:"bytes,2,opt,name=commit_sha,json=commitSha,proto3" json:"commit_sha,omitempty"`
	FeatureSha    string         `protobuf:"bytes,3,opt,name=feature_sha,json=featureSha,proto3" json:"feature_sha,omitempty"`
	NamespaceName string         `protobuf:"bytes,4,opt,name=namespace_name,json=namespaceName,proto3" json:"namespace_name,omitempty"`
	FeatureName   string         `protobuf:"bytes,5,opt,name=feature_name,json=featureName,proto3" json:"feature_name,omitempty"`
	// A list of context keys (not values) that were provided at runtime.
	ContextKeys []*ContextKey `protobuf:"bytes,6,rep,name=context_keys,json=contextKeys,proto3" json:"context_keys,omitempty"`
	// The node in the tree that contained the final return value of the feature.
	ResultPath []int32 `protobuf:"varint,7,rep,packed,name=result_path,json=resultPath,proto3" json:"result_path,omitempty"`
}

func (x *FlagEvaluationEvent) Reset() {
	*x = FlagEvaluationEvent{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *FlagEvaluationEvent) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*FlagEvaluationEvent) ProtoMessage() {}

func (x *FlagEvaluationEvent) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use FlagEvaluationEvent.ProtoReflect.Descriptor instead.
func (*FlagEvaluationEvent) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{7}
}

func (x *FlagEvaluationEvent) GetRepoKey() *RepositoryKey {
	if x != nil {
		return x.RepoKey
	}
	return nil
}

func (x *FlagEvaluationEvent) GetCommitSha() string {
	if x != nil {
		return x.CommitSha
	}
	return ""
}

func (x *FlagEvaluationEvent) GetFeatureSha() string {
	if x != nil {
		return x.FeatureSha
	}
	return ""
}

func (x *FlagEvaluationEvent) GetNamespaceName() string {
	if x != nil {
		return x.NamespaceName
	}
	return ""
}

func (x *FlagEvaluationEvent) GetFeatureName() string {
	if x != nil {
		return x.FeatureName
	}
	return ""
}

func (x *FlagEvaluationEvent) GetContextKeys() []*ContextKey {
	if x != nil {
		return x.ContextKeys
	}
	return nil
}

func (x *FlagEvaluationEvent) GetResultPath() []int32 {
	if x != nil {
		return x.ResultPath
	}
	return nil
}

type SendFlagEvaluationMetricsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Events []*FlagEvaluationEvent `protobuf:"bytes,1,rep,name=events,proto3" json:"events,omitempty"`
}

func (x *SendFlagEvaluationMetricsRequest) Reset() {
	*x = SendFlagEvaluationMetricsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendFlagEvaluationMetricsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendFlagEvaluationMetricsRequest) ProtoMessage() {}

func (x *SendFlagEvaluationMetricsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendFlagEvaluationMetricsRequest.ProtoReflect.Descriptor instead.
func (*SendFlagEvaluationMetricsRequest) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{8}
}

func (x *SendFlagEvaluationMetricsRequest) GetEvents() []*FlagEvaluationEvent {
	if x != nil {
		return x.Events
	}
	return nil
}

type SendFlagEvaluationMetricsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *SendFlagEvaluationMetricsResponse) Reset() {
	*x = SendFlagEvaluationMetricsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SendFlagEvaluationMetricsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SendFlagEvaluationMetricsResponse) ProtoMessage() {}

func (x *SendFlagEvaluationMetricsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SendFlagEvaluationMetricsResponse.ProtoReflect.Descriptor instead.
func (*SendFlagEvaluationMetricsResponse) Descriptor() ([]byte, []int) {
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP(), []int{9}
}

var File_lekko_backend_v1beta1_distribution_service_proto protoreflect.FileDescriptor

var file_lekko_backend_v1beta1_distribution_service_proto_rawDesc = []byte{
	0x0a, 0x30, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2f,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x64, 0x69, 0x73, 0x74, 0x72, 0x69, 0x62, 0x75,
	0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x15, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e,
	0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x1a, 0x31, 0x6c, 0x65, 0x6b, 0x6b, 0x6f,
	0x2f, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x73,
	0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x23, 0x6c, 0x65,
	0x6b, 0x6b, 0x6f, 0x2f, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x2f, 0x76, 0x31, 0x62, 0x65,
	0x74, 0x61, 0x31, 0x2f, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x22, 0x5e, 0x0a, 0x1b, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f,
	0x72, 0x79, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x3f, 0x0a, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65,
	0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x73,
	0x69, 0x74, 0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70, 0x6f, 0x4b, 0x65,
	0x79, 0x22, 0x3d, 0x0a, 0x1c, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f,
	0x72, 0x79, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73,
	0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x5f, 0x73, 0x68, 0x61, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x53, 0x68, 0x61,
	0x22, 0xa9, 0x01, 0x0a, 0x1c, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f,
	0x72, 0x79, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x12, 0x3f, 0x0a, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f,
	0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70, 0x6f, 0x4b,
	0x65, 0x79, 0x12, 0x25, 0x0a, 0x0e, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x5f,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x6e, 0x61, 0x6d, 0x65,
	0x73, 0x70, 0x61, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x66, 0x65, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x0b, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x80, 0x01, 0x0a,
	0x1d, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x43, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1d,
	0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x5f, 0x73, 0x68, 0x61, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x53, 0x68, 0x61, 0x12, 0x40, 0x0a,
	0x0a, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28,
	0x0b, 0x32, 0x20, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e,
	0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x52, 0x0a, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x73, 0x22,
	0x5b, 0x0a, 0x09, 0x4e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x12, 0x12, 0x0a, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x12, 0x3a, 0x0a, 0x08, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65,
	0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x46, 0x65, 0x61, 0x74, 0x75,
	0x72, 0x65, 0x52, 0x08, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x73, 0x22, 0x69, 0x0a, 0x07,
	0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x73,
	0x68, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x73, 0x68, 0x61, 0x12, 0x38, 0x0a,
	0x07, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e,
	0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x46, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x52, 0x07,
	0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0x32, 0x0a, 0x0a, 0x43, 0x6f, 0x6e, 0x74, 0x65,
	0x78, 0x74, 0x4b, 0x65, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x12, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x74, 0x79, 0x70, 0x65, 0x22, 0xc7, 0x02, 0x0a, 0x13,
	0x46, 0x6c, 0x61, 0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x45, 0x76,
	0x65, 0x6e, 0x74, 0x12, 0x3f, 0x0a, 0x08, 0x72, 0x65, 0x70, 0x6f, 0x5f, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61,
	0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x52, 0x65,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52, 0x07, 0x72, 0x65, 0x70,
	0x6f, 0x4b, 0x65, 0x79, 0x12, 0x1d, 0x0a, 0x0a, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x5f, 0x73,
	0x68, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x09, 0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74,
	0x53, 0x68, 0x61, 0x12, 0x1f, 0x0a, 0x0b, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x73,
	0x68, 0x61, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72,
	0x65, 0x53, 0x68, 0x61, 0x12, 0x25, 0x0a, 0x0e, 0x6e, 0x61, 0x6d, 0x65, 0x73, 0x70, 0x61, 0x63,
	0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x6e, 0x61,
	0x6d, 0x65, 0x73, 0x70, 0x61, 0x63, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x66,
	0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x0b, 0x66, 0x65, 0x61, 0x74, 0x75, 0x72, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x44,
	0x0a, 0x0c, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x06,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63,
	0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x43, 0x6f, 0x6e,
	0x74, 0x65, 0x78, 0x74, 0x4b, 0x65, 0x79, 0x52, 0x0b, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74,
	0x4b, 0x65, 0x79, 0x73, 0x12, 0x1f, 0x0a, 0x0b, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x5f, 0x70,
	0x61, 0x74, 0x68, 0x18, 0x07, 0x20, 0x03, 0x28, 0x05, 0x52, 0x0a, 0x72, 0x65, 0x73, 0x75, 0x6c,
	0x74, 0x50, 0x61, 0x74, 0x68, 0x22, 0x66, 0x0a, 0x20, 0x53, 0x65, 0x6e, 0x64, 0x46, 0x6c, 0x61,
	0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x42, 0x0a, 0x06, 0x65, 0x76, 0x65,
	0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x6c, 0x65, 0x6b, 0x6b,
	0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x46, 0x6c, 0x61, 0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x45, 0x76, 0x65, 0x6e, 0x74, 0x52, 0x06, 0x65, 0x76, 0x65, 0x6e, 0x74, 0x73, 0x22, 0x23, 0x0a,
	0x21, 0x53, 0x65, 0x6e, 0x64, 0x46, 0x6c, 0x61, 0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x32, 0xb3, 0x03, 0x0a, 0x13, 0x44, 0x69, 0x73, 0x74, 0x72, 0x69, 0x62, 0x75, 0x74,
	0x69, 0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x81, 0x01, 0x0a, 0x14, 0x47,
	0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x56, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x12, 0x32, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x52,
	0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x56, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x33, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e,
	0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e,
	0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x56, 0x65, 0x72,
	0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x84,
	0x01, 0x0a, 0x15, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79,
	0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x12, 0x33, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f,
	0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x43, 0x6f,
	0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x34, 0x2e,
	0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x6f, 0x72, 0x79, 0x43, 0x6f, 0x6e, 0x74, 0x65, 0x6e, 0x74, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x90, 0x01, 0x0a, 0x19, 0x53, 0x65, 0x6e, 0x64, 0x46, 0x6c,
	0x61, 0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x12, 0x37, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b,
	0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x53, 0x65, 0x6e, 0x64,
	0x46, 0x6c, 0x61, 0x67, 0x45, 0x76, 0x61, 0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x38, 0x2e, 0x6c,
	0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x2e, 0x53, 0x65, 0x6e, 0x64, 0x46, 0x6c, 0x61, 0x67, 0x45, 0x76, 0x61,
	0x6c, 0x75, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0xfa, 0x01, 0x0a, 0x19, 0x63, 0x6f, 0x6d,
	0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42, 0x18, 0x44, 0x69, 0x73, 0x74, 0x72, 0x69, 0x62, 0x75,
	0x74, 0x69, 0x6f, 0x6e, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x50, 0x72, 0x6f, 0x74, 0x6f,
	0x50, 0x01, 0x5a, 0x4d, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c,
	0x65, 0x6b, 0x6b, 0x6f, 0x64, 0x65, 0x76, 0x2f, 0x63, 0x6c, 0x69, 0x2f, 0x70, 0x6b, 0x67, 0x2f,
	0x67, 0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x2f, 0x6c, 0x65, 0x6b,
	0x6b, 0x6f, 0x2f, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0x3b, 0x62, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0xa2, 0x02, 0x03, 0x4c, 0x42, 0x58, 0xaa, 0x02, 0x15, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e,
	0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x2e, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0xca,
	0x02, 0x15, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x5c, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x5c,
	0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02, 0x21, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x5c,
	0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x5c,
	0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x17, 0x4c, 0x65,
	0x6b, 0x6b, 0x6f, 0x3a, 0x3a, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x3a, 0x3a, 0x56, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_lekko_backend_v1beta1_distribution_service_proto_rawDescOnce sync.Once
	file_lekko_backend_v1beta1_distribution_service_proto_rawDescData = file_lekko_backend_v1beta1_distribution_service_proto_rawDesc
)

func file_lekko_backend_v1beta1_distribution_service_proto_rawDescGZIP() []byte {
	file_lekko_backend_v1beta1_distribution_service_proto_rawDescOnce.Do(func() {
		file_lekko_backend_v1beta1_distribution_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_lekko_backend_v1beta1_distribution_service_proto_rawDescData)
	})
	return file_lekko_backend_v1beta1_distribution_service_proto_rawDescData
}

var file_lekko_backend_v1beta1_distribution_service_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_lekko_backend_v1beta1_distribution_service_proto_goTypes = []interface{}{
	(*GetRepositoryVersionRequest)(nil),       // 0: lekko.backend.v1beta1.GetRepositoryVersionRequest
	(*GetRepositoryVersionResponse)(nil),      // 1: lekko.backend.v1beta1.GetRepositoryVersionResponse
	(*GetRepositoryContentsRequest)(nil),      // 2: lekko.backend.v1beta1.GetRepositoryContentsRequest
	(*GetRepositoryContentsResponse)(nil),     // 3: lekko.backend.v1beta1.GetRepositoryContentsResponse
	(*Namespace)(nil),                         // 4: lekko.backend.v1beta1.Namespace
	(*Feature)(nil),                           // 5: lekko.backend.v1beta1.Feature
	(*ContextKey)(nil),                        // 6: lekko.backend.v1beta1.ContextKey
	(*FlagEvaluationEvent)(nil),               // 7: lekko.backend.v1beta1.FlagEvaluationEvent
	(*SendFlagEvaluationMetricsRequest)(nil),  // 8: lekko.backend.v1beta1.SendFlagEvaluationMetricsRequest
	(*SendFlagEvaluationMetricsResponse)(nil), // 9: lekko.backend.v1beta1.SendFlagEvaluationMetricsResponse
	(*RepositoryKey)(nil),                     // 10: lekko.backend.v1beta1.RepositoryKey
	(*v1beta1.Feature)(nil),                   // 11: lekko.feature.v1beta1.Feature
}
var file_lekko_backend_v1beta1_distribution_service_proto_depIdxs = []int32{
	10, // 0: lekko.backend.v1beta1.GetRepositoryVersionRequest.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	10, // 1: lekko.backend.v1beta1.GetRepositoryContentsRequest.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	4,  // 2: lekko.backend.v1beta1.GetRepositoryContentsResponse.namespaces:type_name -> lekko.backend.v1beta1.Namespace
	5,  // 3: lekko.backend.v1beta1.Namespace.features:type_name -> lekko.backend.v1beta1.Feature
	11, // 4: lekko.backend.v1beta1.Feature.feature:type_name -> lekko.feature.v1beta1.Feature
	10, // 5: lekko.backend.v1beta1.FlagEvaluationEvent.repo_key:type_name -> lekko.backend.v1beta1.RepositoryKey
	6,  // 6: lekko.backend.v1beta1.FlagEvaluationEvent.context_keys:type_name -> lekko.backend.v1beta1.ContextKey
	7,  // 7: lekko.backend.v1beta1.SendFlagEvaluationMetricsRequest.events:type_name -> lekko.backend.v1beta1.FlagEvaluationEvent
	0,  // 8: lekko.backend.v1beta1.DistributionService.GetRepositoryVersion:input_type -> lekko.backend.v1beta1.GetRepositoryVersionRequest
	2,  // 9: lekko.backend.v1beta1.DistributionService.GetRepositoryContents:input_type -> lekko.backend.v1beta1.GetRepositoryContentsRequest
	8,  // 10: lekko.backend.v1beta1.DistributionService.SendFlagEvaluationMetrics:input_type -> lekko.backend.v1beta1.SendFlagEvaluationMetricsRequest
	1,  // 11: lekko.backend.v1beta1.DistributionService.GetRepositoryVersion:output_type -> lekko.backend.v1beta1.GetRepositoryVersionResponse
	3,  // 12: lekko.backend.v1beta1.DistributionService.GetRepositoryContents:output_type -> lekko.backend.v1beta1.GetRepositoryContentsResponse
	9,  // 13: lekko.backend.v1beta1.DistributionService.SendFlagEvaluationMetrics:output_type -> lekko.backend.v1beta1.SendFlagEvaluationMetricsResponse
	11, // [11:14] is the sub-list for method output_type
	8,  // [8:11] is the sub-list for method input_type
	8,  // [8:8] is the sub-list for extension type_name
	8,  // [8:8] is the sub-list for extension extendee
	0,  // [0:8] is the sub-list for field type_name
}

func init() { file_lekko_backend_v1beta1_distribution_service_proto_init() }
func file_lekko_backend_v1beta1_distribution_service_proto_init() {
	if File_lekko_backend_v1beta1_distribution_service_proto != nil {
		return
	}
	file_lekko_backend_v1beta1_configuration_service_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetRepositoryVersionRequest); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetRepositoryVersionResponse); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetRepositoryContentsRequest); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetRepositoryContentsResponse); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Namespace); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Feature); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ContextKey); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*FlagEvaluationEvent); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendFlagEvaluationMetricsRequest); i {
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
		file_lekko_backend_v1beta1_distribution_service_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SendFlagEvaluationMetricsResponse); i {
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
			RawDescriptor: file_lekko_backend_v1beta1_distribution_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_lekko_backend_v1beta1_distribution_service_proto_goTypes,
		DependencyIndexes: file_lekko_backend_v1beta1_distribution_service_proto_depIdxs,
		MessageInfos:      file_lekko_backend_v1beta1_distribution_service_proto_msgTypes,
	}.Build()
	File_lekko_backend_v1beta1_distribution_service_proto = out.File
	file_lekko_backend_v1beta1_distribution_service_proto_rawDesc = nil
	file_lekko_backend_v1beta1_distribution_service_proto_goTypes = nil
	file_lekko_backend_v1beta1_distribution_service_proto_depIdxs = nil
}

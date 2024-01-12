// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        (unknown)
// source: default/config/v1beta1/default.proto

package configv1beta1

import (
	v1beta1 "github.com/lekkodev/cli/internal/lekko/proto/lekko/bff/v1beta1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	durationpb "google.golang.org/protobuf/types/known/durationpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// LogLevel used to denote how a message should be logged
// via configuration.
type LogLevel int32

const (
	LogLevel_LOG_LEVEL_UNSPECIFIED LogLevel = 0
	LogLevel_LOG_LEVEL_ERROR       LogLevel = 1
	LogLevel_LOG_LEVEL_WARNING     LogLevel = 2
	LogLevel_LOG_LEVEL_INFO        LogLevel = 3
	LogLevel_LOG_LEVEL_DEBUG       LogLevel = 4
)

// Enum value maps for LogLevel.
var (
	LogLevel_name = map[int32]string{
		0: "LOG_LEVEL_UNSPECIFIED",
		1: "LOG_LEVEL_ERROR",
		2: "LOG_LEVEL_WARNING",
		3: "LOG_LEVEL_INFO",
		4: "LOG_LEVEL_DEBUG",
	}
	LogLevel_value = map[string]int32{
		"LOG_LEVEL_UNSPECIFIED": 0,
		"LOG_LEVEL_ERROR":       1,
		"LOG_LEVEL_WARNING":     2,
		"LOG_LEVEL_INFO":        3,
		"LOG_LEVEL_DEBUG":       4,
	}
)

func (x LogLevel) Enum() *LogLevel {
	p := new(LogLevel)
	*p = x
	return p
}

func (x LogLevel) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogLevel) Descriptor() protoreflect.EnumDescriptor {
	return file_default_config_v1beta1_default_proto_enumTypes[0].Descriptor()
}

func (LogLevel) Type() protoreflect.EnumType {
	return &file_default_config_v1beta1_default_proto_enumTypes[0]
}

func (x LogLevel) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogLevel.Descriptor instead.
func (LogLevel) EnumDescriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{0}
}

type MiddlewareConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Procedures that don't require a team name in the auth
	// context.
	TeamExemptProcedures map[string]bool `protobuf:"bytes,1,rep,name=team_exempt_procedures,json=teamExemptProcedures,proto3" json:"team_exempt_procedures,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
	// Procedures that require an oauth token (e.g. github)
	//
	//	in the auth context.
	RequireOauthProcedures map[string]bool `protobuf:"bytes,2,rep,name=require_oauth_procedures,json=requireOauthProcedures,proto3" json:"require_oauth_procedures,omitempty" protobuf_key:"bytes,1,opt,name=key,proto3" protobuf_val:"varint,2,opt,name=value,proto3"`
}

func (x *MiddlewareConfig) Reset() {
	*x = MiddlewareConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MiddlewareConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MiddlewareConfig) ProtoMessage() {}

func (x *MiddlewareConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MiddlewareConfig.ProtoReflect.Descriptor instead.
func (*MiddlewareConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{0}
}

func (x *MiddlewareConfig) GetTeamExemptProcedures() map[string]bool {
	if x != nil {
		return x.TeamExemptProcedures
	}
	return nil
}

func (x *MiddlewareConfig) GetRequireOauthProcedures() map[string]bool {
	if x != nil {
		return x.RequireOauthProcedures
	}
	return nil
}

// Configuration around Lekko's OAuth 2.0 Device authorization flow.
type OAuthDeviceConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	VerificationUri        string `protobuf:"bytes,1,opt,name=verification_uri,json=verificationUri,proto3" json:"verification_uri,omitempty"`
	PollingIntervalSeconds int64  `protobuf:"varint,2,opt,name=polling_interval_seconds,json=pollingIntervalSeconds,proto3" json:"polling_interval_seconds,omitempty"`
}

func (x *OAuthDeviceConfig) Reset() {
	*x = OAuthDeviceConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *OAuthDeviceConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*OAuthDeviceConfig) ProtoMessage() {}

func (x *OAuthDeviceConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use OAuthDeviceConfig.ProtoReflect.Descriptor instead.
func (*OAuthDeviceConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{1}
}

func (x *OAuthDeviceConfig) GetVerificationUri() string {
	if x != nil {
		return x.VerificationUri
	}
	return ""
}

func (x *OAuthDeviceConfig) GetPollingIntervalSeconds() int64 {
	if x != nil {
		return x.PollingIntervalSeconds
	}
	return 0
}

// Config around how rollouts are set up in the backend.
type RolloutConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// How long a rollout can hold a lock on a particular repository
	LockTtl *durationpb.Duration `protobuf:"bytes,1,opt,name=lock_ttl,json=lockTtl,proto3" json:"lock_ttl,omitempty"`
	// The number of background workers performing rollouts concurrently
	NumRolloutWorkers int64 `protobuf:"varint,2,opt,name=num_rollout_workers,json=numRolloutWorkers,proto3" json:"num_rollout_workers,omitempty"`
	// The duration after which a single rollout times out.
	RolloutTimeout *durationpb.Duration          `protobuf:"bytes,3,opt,name=rollout_timeout,json=rolloutTimeout,proto3" json:"rollout_timeout,omitempty"`
	Background     *RolloutConfig_BackgroundLoop `protobuf:"bytes,4,opt,name=background,proto3" json:"background,omitempty"`
	// The size of the rollout channel buffer. Updating
	// this value requires a restart.
	ChanBufferSize int64 `protobuf:"varint,5,opt,name=chan_buffer_size,json=chanBufferSize,proto3" json:"chan_buffer_size,omitempty"`
}

func (x *RolloutConfig) Reset() {
	*x = RolloutConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RolloutConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RolloutConfig) ProtoMessage() {}

func (x *RolloutConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RolloutConfig.ProtoReflect.Descriptor instead.
func (*RolloutConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{2}
}

func (x *RolloutConfig) GetLockTtl() *durationpb.Duration {
	if x != nil {
		return x.LockTtl
	}
	return nil
}

func (x *RolloutConfig) GetNumRolloutWorkers() int64 {
	if x != nil {
		return x.NumRolloutWorkers
	}
	return 0
}

func (x *RolloutConfig) GetRolloutTimeout() *durationpb.Duration {
	if x != nil {
		return x.RolloutTimeout
	}
	return nil
}

func (x *RolloutConfig) GetBackground() *RolloutConfig_BackgroundLoop {
	if x != nil {
		return x.Background
	}
	return nil
}

func (x *RolloutConfig) GetChanBufferSize() int64 {
	if x != nil {
		return x.ChanBufferSize
	}
	return 0
}

type RepositoryConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CriticalRepositories []*v1beta1.RepositoryKey `protobuf:"bytes,1,rep,name=critical_repositories,json=criticalRepositories,proto3" json:"critical_repositories,omitempty"`
}

func (x *RepositoryConfig) Reset() {
	*x = RepositoryConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RepositoryConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RepositoryConfig) ProtoMessage() {}

func (x *RepositoryConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RepositoryConfig.ProtoReflect.Descriptor instead.
func (*RepositoryConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{3}
}

func (x *RepositoryConfig) GetCriticalRepositories() []*v1beta1.RepositoryKey {
	if x != nil {
		return x.CriticalRepositories
	}
	return nil
}

// ErrorFilter contains information about how to log errors in middleware.
type ErrorFilter struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	LogLevel LogLevel `protobuf:"varint,1,opt,name=log_level,json=logLevel,proto3,enum=default.config.v1beta1.LogLevel" json:"log_level,omitempty"`
}

func (x *ErrorFilter) Reset() {
	*x = ErrorFilter{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ErrorFilter) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ErrorFilter) ProtoMessage() {}

func (x *ErrorFilter) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ErrorFilter.ProtoReflect.Descriptor instead.
func (*ErrorFilter) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{4}
}

func (x *ErrorFilter) GetLogLevel() LogLevel {
	if x != nil {
		return x.LogLevel
	}
	return LogLevel_LOG_LEVEL_UNSPECIFIED
}

// Settings for Memcached client
type MemcachedConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// How many idle connections to keep open to Memcached without closing them.
	// Should be increased for heavy concurrent load to prevent timeouts.
	MaxIdleConns int64 `protobuf:"varint,1,opt,name=max_idle_conns,json=maxIdleConns,proto3" json:"max_idle_conns,omitempty"`
	// Maximum timeout before aborting Memcached calls
	TimeoutMs int64 `protobuf:"varint,2,opt,name=timeout_ms,json=timeoutMs,proto3" json:"timeout_ms,omitempty"`
}

func (x *MemcachedConfig) Reset() {
	*x = MemcachedConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *MemcachedConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*MemcachedConfig) ProtoMessage() {}

func (x *MemcachedConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use MemcachedConfig.ProtoReflect.Descriptor instead.
func (*MemcachedConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{5}
}

func (x *MemcachedConfig) GetMaxIdleConns() int64 {
	if x != nil {
		return x.MaxIdleConns
	}
	return 0
}

func (x *MemcachedConfig) GetTimeoutMs() int64 {
	if x != nil {
		return x.TimeoutMs
	}
	return 0
}

type DBConfig struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// How many idle connections to keep open to the DB without closing them.
	// Essentially determines the size of the connection pool of reusable
	// connections.
	MaxIdleConns int64 `protobuf:"varint,1,opt,name=max_idle_conns,json=maxIdleConns,proto3" json:"max_idle_conns,omitempty"`
	// How many maximum open connections to have to the DB at once.
	// Should be tuned if too many connections cause CPU/memory overload.
	// If not set, should be interpreted as unlimited.
	MaxOpenConns int64 `protobuf:"varint,2,opt,name=max_open_conns,json=maxOpenConns,proto3" json:"max_open_conns,omitempty"`
}

func (x *DBConfig) Reset() {
	*x = DBConfig{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DBConfig) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DBConfig) ProtoMessage() {}

func (x *DBConfig) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DBConfig.ProtoReflect.Descriptor instead.
func (*DBConfig) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{6}
}

func (x *DBConfig) GetMaxIdleConns() int64 {
	if x != nil {
		return x.MaxIdleConns
	}
	return 0
}

func (x *DBConfig) GetMaxOpenConns() int64 {
	if x != nil {
		return x.MaxOpenConns
	}
	return 0
}

type BackendContext struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Env      string `protobuf:"bytes,1,opt,name=env,proto3" json:"env,omitempty"`
	TeamName string `protobuf:"bytes,2,opt,name=team_name,json=teamName,proto3" json:"team_name,omitempty"`
}

func (x *BackendContext) Reset() {
	*x = BackendContext{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BackendContext) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BackendContext) ProtoMessage() {}

func (x *BackendContext) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BackendContext.ProtoReflect.Descriptor instead.
func (*BackendContext) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{7}
}

func (x *BackendContext) GetEnv() string {
	if x != nil {
		return x.Env
	}
	return ""
}

func (x *BackendContext) GetTeamName() string {
	if x != nil {
		return x.TeamName
	}
	return ""
}

type RolloutConfig_BackgroundLoop struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Delay  *durationpb.Duration `protobuf:"bytes,1,opt,name=delay,proto3" json:"delay,omitempty"`
	Jitter *durationpb.Duration `protobuf:"bytes,2,opt,name=jitter,proto3" json:"jitter,omitempty"`
}

func (x *RolloutConfig_BackgroundLoop) Reset() {
	*x = RolloutConfig_BackgroundLoop{}
	if protoimpl.UnsafeEnabled {
		mi := &file_default_config_v1beta1_default_proto_msgTypes[10]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RolloutConfig_BackgroundLoop) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RolloutConfig_BackgroundLoop) ProtoMessage() {}

func (x *RolloutConfig_BackgroundLoop) ProtoReflect() protoreflect.Message {
	mi := &file_default_config_v1beta1_default_proto_msgTypes[10]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RolloutConfig_BackgroundLoop.ProtoReflect.Descriptor instead.
func (*RolloutConfig_BackgroundLoop) Descriptor() ([]byte, []int) {
	return file_default_config_v1beta1_default_proto_rawDescGZIP(), []int{2, 0}
}

func (x *RolloutConfig_BackgroundLoop) GetDelay() *durationpb.Duration {
	if x != nil {
		return x.Delay
	}
	return nil
}

func (x *RolloutConfig_BackgroundLoop) GetJitter() *durationpb.Duration {
	if x != nil {
		return x.Jitter
	}
	return nil
}

var File_default_config_v1beta1_default_proto protoreflect.FileDescriptor

var file_default_config_v1beta1_default_proto_rawDesc = []byte{
	0x0a, 0x24, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2f, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x16, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x2e,
	0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x1a, 0x1e,
	0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f,
	0x64, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b,
	0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x62, 0x66, 0x66, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2f, 0x62, 0x66, 0x66, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xa0, 0x03, 0x0a, 0x10,
	0x4d, 0x69, 0x64, 0x64, 0x6c, 0x65, 0x77, 0x61, 0x72, 0x65, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67,
	0x12, 0x78, 0x0a, 0x16, 0x74, 0x65, 0x61, 0x6d, 0x5f, 0x65, 0x78, 0x65, 0x6d, 0x70, 0x74, 0x5f,
	0x70, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x42, 0x2e, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69,
	0x67, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4d, 0x69, 0x64, 0x64, 0x6c, 0x65,
	0x77, 0x61, 0x72, 0x65, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x54, 0x65, 0x61, 0x6d, 0x45,
	0x78, 0x65, 0x6d, 0x70, 0x74, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x45,
	0x6e, 0x74, 0x72, 0x79, 0x52, 0x14, 0x74, 0x65, 0x61, 0x6d, 0x45, 0x78, 0x65, 0x6d, 0x70, 0x74,
	0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x12, 0x7e, 0x0a, 0x18, 0x72, 0x65,
	0x71, 0x75, 0x69, 0x72, 0x65, 0x5f, 0x6f, 0x61, 0x75, 0x74, 0x68, 0x5f, 0x70, 0x72, 0x6f, 0x63,
	0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x44, 0x2e, 0x64,
	0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x2e, 0x4d, 0x69, 0x64, 0x64, 0x6c, 0x65, 0x77, 0x61, 0x72, 0x65,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x52, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x4f, 0x61,
	0x75, 0x74, 0x68, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x52, 0x16, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x4f, 0x61, 0x75, 0x74, 0x68,
	0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x1a, 0x47, 0x0a, 0x19, 0x54, 0x65,
	0x61, 0x6d, 0x45, 0x78, 0x65, 0x6d, 0x70, 0x74, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72,
	0x65, 0x73, 0x45, 0x6e, 0x74, 0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c,
	0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x08, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a,
	0x02, 0x38, 0x01, 0x1a, 0x49, 0x0a, 0x1b, 0x52, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x4f, 0x61,
	0x75, 0x74, 0x68, 0x50, 0x72, 0x6f, 0x63, 0x65, 0x64, 0x75, 0x72, 0x65, 0x73, 0x45, 0x6e, 0x74,
	0x72, 0x79, 0x12, 0x10, 0x0a, 0x03, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x03, 0x6b, 0x65, 0x79, 0x12, 0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20,
	0x01, 0x28, 0x08, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x3a, 0x02, 0x38, 0x01, 0x22, 0x78,
	0x0a, 0x11, 0x4f, 0x41, 0x75, 0x74, 0x68, 0x44, 0x65, 0x76, 0x69, 0x63, 0x65, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x12, 0x29, 0x0a, 0x10, 0x76, 0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x5f, 0x75, 0x72, 0x69, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0f, 0x76,
	0x65, 0x72, 0x69, 0x66, 0x69, 0x63, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x55, 0x72, 0x69, 0x12, 0x38,
	0x0a, 0x18, 0x70, 0x6f, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x5f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x76,
	0x61, 0x6c, 0x5f, 0x73, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x16, 0x70, 0x6f, 0x6c, 0x6c, 0x69, 0x6e, 0x67, 0x49, 0x6e, 0x74, 0x65, 0x72, 0x76, 0x61,
	0x6c, 0x53, 0x65, 0x63, 0x6f, 0x6e, 0x64, 0x73, 0x22, 0xaf, 0x03, 0x0a, 0x0d, 0x52, 0x6f, 0x6c,
	0x6c, 0x6f, 0x75, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x34, 0x0a, 0x08, 0x6c, 0x6f,
	0x63, 0x6b, 0x5f, 0x74, 0x74, 0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67,
	0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44,
	0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x52, 0x07, 0x6c, 0x6f, 0x63, 0x6b, 0x54, 0x74, 0x6c,
	0x12, 0x2e, 0x0a, 0x13, 0x6e, 0x75, 0x6d, 0x5f, 0x72, 0x6f, 0x6c, 0x6c, 0x6f, 0x75, 0x74, 0x5f,
	0x77, 0x6f, 0x72, 0x6b, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x11, 0x6e,
	0x75, 0x6d, 0x52, 0x6f, 0x6c, 0x6c, 0x6f, 0x75, 0x74, 0x57, 0x6f, 0x72, 0x6b, 0x65, 0x72, 0x73,
	0x12, 0x42, 0x0a, 0x0f, 0x72, 0x6f, 0x6c, 0x6c, 0x6f, 0x75, 0x74, 0x5f, 0x74, 0x69, 0x6d, 0x65,
	0x6f, 0x75, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67,
	0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x52, 0x0e, 0x72, 0x6f, 0x6c, 0x6c, 0x6f, 0x75, 0x74, 0x54, 0x69, 0x6d,
	0x65, 0x6f, 0x75, 0x74, 0x12, 0x54, 0x0a, 0x0a, 0x62, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75,
	0x6e, 0x64, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x34, 0x2e, 0x64, 0x65, 0x66, 0x61, 0x75,
	0x6c, 0x74, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x52, 0x6f, 0x6c, 0x6c, 0x6f, 0x75, 0x74, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e,
	0x42, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75, 0x6e, 0x64, 0x4c, 0x6f, 0x6f, 0x70, 0x52, 0x0a,
	0x62, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75, 0x6e, 0x64, 0x12, 0x28, 0x0a, 0x10, 0x63, 0x68,
	0x61, 0x6e, 0x5f, 0x62, 0x75, 0x66, 0x66, 0x65, 0x72, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x05,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x0e, 0x63, 0x68, 0x61, 0x6e, 0x42, 0x75, 0x66, 0x66, 0x65, 0x72,
	0x53, 0x69, 0x7a, 0x65, 0x1a, 0x74, 0x0a, 0x0e, 0x42, 0x61, 0x63, 0x6b, 0x67, 0x72, 0x6f, 0x75,
	0x6e, 0x64, 0x4c, 0x6f, 0x6f, 0x70, 0x12, 0x2f, 0x0a, 0x05, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x52, 0x05, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x12, 0x31, 0x0a, 0x06, 0x6a, 0x69, 0x74, 0x74, 0x65,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x44, 0x75, 0x72, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x52, 0x06, 0x6a, 0x69, 0x74, 0x74, 0x65, 0x72, 0x22, 0x69, 0x0a, 0x10, 0x52, 0x65,
	0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x55,
	0x0a, 0x15, 0x63, 0x72, 0x69, 0x74, 0x69, 0x63, 0x61, 0x6c, 0x5f, 0x72, 0x65, 0x70, 0x6f, 0x73,
	0x69, 0x74, 0x6f, 0x72, 0x69, 0x65, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x20, 0x2e,
	0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x62, 0x66, 0x66, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0x2e, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x6f, 0x72, 0x79, 0x4b, 0x65, 0x79, 0x52,
	0x14, 0x63, 0x72, 0x69, 0x74, 0x69, 0x63, 0x61, 0x6c, 0x52, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x6f, 0x72, 0x69, 0x65, 0x73, 0x22, 0x4c, 0x0a, 0x0b, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x46, 0x69,
	0x6c, 0x74, 0x65, 0x72, 0x12, 0x3d, 0x0a, 0x09, 0x6c, 0x6f, 0x67, 0x5f, 0x6c, 0x65, 0x76, 0x65,
	0x6c, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x20, 0x2e, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c,
	0x74, 0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31,
	0x2e, 0x4c, 0x6f, 0x67, 0x4c, 0x65, 0x76, 0x65, 0x6c, 0x52, 0x08, 0x6c, 0x6f, 0x67, 0x4c, 0x65,
	0x76, 0x65, 0x6c, 0x22, 0x56, 0x0a, 0x0f, 0x4d, 0x65, 0x6d, 0x63, 0x61, 0x63, 0x68, 0x65, 0x64,
	0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x24, 0x0a, 0x0e, 0x6d, 0x61, 0x78, 0x5f, 0x69, 0x64,
	0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0c,
	0x6d, 0x61, 0x78, 0x49, 0x64, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x73, 0x12, 0x1d, 0x0a, 0x0a,
	0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x5f, 0x6d, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x09, 0x74, 0x69, 0x6d, 0x65, 0x6f, 0x75, 0x74, 0x4d, 0x73, 0x22, 0x56, 0x0a, 0x08, 0x44,
	0x42, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x12, 0x24, 0x0a, 0x0e, 0x6d, 0x61, 0x78, 0x5f, 0x69,
	0x64, 0x6c, 0x65, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52,
	0x0c, 0x6d, 0x61, 0x78, 0x49, 0x64, 0x6c, 0x65, 0x43, 0x6f, 0x6e, 0x6e, 0x73, 0x12, 0x24, 0x0a,
	0x0e, 0x6d, 0x61, 0x78, 0x5f, 0x6f, 0x70, 0x65, 0x6e, 0x5f, 0x63, 0x6f, 0x6e, 0x6e, 0x73, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0c, 0x6d, 0x61, 0x78, 0x4f, 0x70, 0x65, 0x6e, 0x43, 0x6f,
	0x6e, 0x6e, 0x73, 0x22, 0x3f, 0x0a, 0x0e, 0x42, 0x61, 0x63, 0x6b, 0x65, 0x6e, 0x64, 0x43, 0x6f,
	0x6e, 0x74, 0x65, 0x78, 0x74, 0x12, 0x10, 0x0a, 0x03, 0x65, 0x6e, 0x76, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x03, 0x65, 0x6e, 0x76, 0x12, 0x1b, 0x0a, 0x09, 0x74, 0x65, 0x61, 0x6d, 0x5f,
	0x6e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x74, 0x65, 0x61, 0x6d,
	0x4e, 0x61, 0x6d, 0x65, 0x2a, 0x7a, 0x0a, 0x08, 0x4c, 0x6f, 0x67, 0x4c, 0x65, 0x76, 0x65, 0x6c,
	0x12, 0x19, 0x0a, 0x15, 0x4c, 0x4f, 0x47, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x55, 0x4e,
	0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x13, 0x0a, 0x0f, 0x4c,
	0x4f, 0x47, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x01,
	0x12, 0x15, 0x0a, 0x11, 0x4c, 0x4f, 0x47, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x57, 0x41,
	0x52, 0x4e, 0x49, 0x4e, 0x47, 0x10, 0x02, 0x12, 0x12, 0x0a, 0x0e, 0x4c, 0x4f, 0x47, 0x5f, 0x4c,
	0x45, 0x56, 0x45, 0x4c, 0x5f, 0x49, 0x4e, 0x46, 0x4f, 0x10, 0x03, 0x12, 0x13, 0x0a, 0x0f, 0x4c,
	0x4f, 0x47, 0x5f, 0x4c, 0x45, 0x56, 0x45, 0x4c, 0x5f, 0x44, 0x45, 0x42, 0x55, 0x47, 0x10, 0x04,
	0x42, 0xf9, 0x01, 0x0a, 0x1a, 0x63, 0x6f, 0x6d, 0x2e, 0x64, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74,
	0x2e, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x42,
	0x0c, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a,
	0x51, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x65, 0x6b, 0x6b,
	0x6f, 0x64, 0x65, 0x76, 0x2f, 0x63, 0x6c, 0x69, 0x2f, 0x69, 0x6e, 0x74, 0x65, 0x72, 0x6e, 0x61,
	0x6c, 0x2f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x64, 0x65,
	0x66, 0x61, 0x75, 0x6c, 0x74, 0x2f, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2f, 0x76, 0x31, 0x62,
	0x65, 0x74, 0x61, 0x31, 0x3b, 0x63, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x76, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x31, 0xa2, 0x02, 0x03, 0x44, 0x43, 0x58, 0xaa, 0x02, 0x16, 0x44, 0x65, 0x66, 0x61, 0x75,
	0x6c, 0x74, 0x2e, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x2e, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x31, 0xca, 0x02, 0x17, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x5c, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0xe2, 0x02, 0x23, 0x44, 0x65,
	0x66, 0x61, 0x75, 0x6c, 0x74, 0x5f, 0x5c, 0x43, 0x6f, 0x6e, 0x66, 0x69, 0x67, 0x5c, 0x56, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x31, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74,
	0x61, 0xea, 0x02, 0x18, 0x44, 0x65, 0x66, 0x61, 0x75, 0x6c, 0x74, 0x3a, 0x3a, 0x43, 0x6f, 0x6e,
	0x66, 0x69, 0x67, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x31, 0x62, 0x06, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_default_config_v1beta1_default_proto_rawDescOnce sync.Once
	file_default_config_v1beta1_default_proto_rawDescData = file_default_config_v1beta1_default_proto_rawDesc
)

func file_default_config_v1beta1_default_proto_rawDescGZIP() []byte {
	file_default_config_v1beta1_default_proto_rawDescOnce.Do(func() {
		file_default_config_v1beta1_default_proto_rawDescData = protoimpl.X.CompressGZIP(file_default_config_v1beta1_default_proto_rawDescData)
	})
	return file_default_config_v1beta1_default_proto_rawDescData
}

var file_default_config_v1beta1_default_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_default_config_v1beta1_default_proto_msgTypes = make([]protoimpl.MessageInfo, 11)
var file_default_config_v1beta1_default_proto_goTypes = []interface{}{
	(LogLevel)(0),                        // 0: default.config.v1beta1.LogLevel
	(*MiddlewareConfig)(nil),             // 1: default.config.v1beta1.MiddlewareConfig
	(*OAuthDeviceConfig)(nil),            // 2: default.config.v1beta1.OAuthDeviceConfig
	(*RolloutConfig)(nil),                // 3: default.config.v1beta1.RolloutConfig
	(*RepositoryConfig)(nil),             // 4: default.config.v1beta1.RepositoryConfig
	(*ErrorFilter)(nil),                  // 5: default.config.v1beta1.ErrorFilter
	(*MemcachedConfig)(nil),              // 6: default.config.v1beta1.MemcachedConfig
	(*DBConfig)(nil),                     // 7: default.config.v1beta1.DBConfig
	(*BackendContext)(nil),               // 8: default.config.v1beta1.BackendContext
	nil,                                  // 9: default.config.v1beta1.MiddlewareConfig.TeamExemptProceduresEntry
	nil,                                  // 10: default.config.v1beta1.MiddlewareConfig.RequireOauthProceduresEntry
	(*RolloutConfig_BackgroundLoop)(nil), // 11: default.config.v1beta1.RolloutConfig.BackgroundLoop
	(*durationpb.Duration)(nil),          // 12: google.protobuf.Duration
	(*v1beta1.RepositoryKey)(nil),        // 13: lekko.bff.v1beta1.RepositoryKey
}
var file_default_config_v1beta1_default_proto_depIdxs = []int32{
	9,  // 0: default.config.v1beta1.MiddlewareConfig.team_exempt_procedures:type_name -> default.config.v1beta1.MiddlewareConfig.TeamExemptProceduresEntry
	10, // 1: default.config.v1beta1.MiddlewareConfig.require_oauth_procedures:type_name -> default.config.v1beta1.MiddlewareConfig.RequireOauthProceduresEntry
	12, // 2: default.config.v1beta1.RolloutConfig.lock_ttl:type_name -> google.protobuf.Duration
	12, // 3: default.config.v1beta1.RolloutConfig.rollout_timeout:type_name -> google.protobuf.Duration
	11, // 4: default.config.v1beta1.RolloutConfig.background:type_name -> default.config.v1beta1.RolloutConfig.BackgroundLoop
	13, // 5: default.config.v1beta1.RepositoryConfig.critical_repositories:type_name -> lekko.bff.v1beta1.RepositoryKey
	0,  // 6: default.config.v1beta1.ErrorFilter.log_level:type_name -> default.config.v1beta1.LogLevel
	12, // 7: default.config.v1beta1.RolloutConfig.BackgroundLoop.delay:type_name -> google.protobuf.Duration
	12, // 8: default.config.v1beta1.RolloutConfig.BackgroundLoop.jitter:type_name -> google.protobuf.Duration
	9,  // [9:9] is the sub-list for method output_type
	9,  // [9:9] is the sub-list for method input_type
	9,  // [9:9] is the sub-list for extension type_name
	9,  // [9:9] is the sub-list for extension extendee
	0,  // [0:9] is the sub-list for field type_name
}

func init() { file_default_config_v1beta1_default_proto_init() }
func file_default_config_v1beta1_default_proto_init() {
	if File_default_config_v1beta1_default_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_default_config_v1beta1_default_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MiddlewareConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*OAuthDeviceConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RolloutConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RepositoryConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ErrorFilter); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*MemcachedConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DBConfig); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BackendContext); i {
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
		file_default_config_v1beta1_default_proto_msgTypes[10].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RolloutConfig_BackgroundLoop); i {
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
			RawDescriptor: file_default_config_v1beta1_default_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   11,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_default_config_v1beta1_default_proto_goTypes,
		DependencyIndexes: file_default_config_v1beta1_default_proto_depIdxs,
		EnumInfos:         file_default_config_v1beta1_default_proto_enumTypes,
		MessageInfos:      file_default_config_v1beta1_default_proto_msgTypes,
	}.Build()
	File_default_config_v1beta1_default_proto = out.File
	file_default_config_v1beta1_default_proto_rawDesc = nil
	file_default_config_v1beta1_default_proto_goTypes = nil
	file_default_config_v1beta1_default_proto_depIdxs = nil
}

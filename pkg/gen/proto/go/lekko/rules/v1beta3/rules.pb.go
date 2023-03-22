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
// source: lekko/rules/v1beta3/rules.proto

package rulesv1beta3

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	structpb "google.golang.org/protobuf/types/known/structpb"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type ComparisonOperator int32

const (
	ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED ComparisonOperator = 0
	// == only applies to number, string and bool values.
	ComparisonOperator_COMPARISON_OPERATOR_EQUALS ComparisonOperator = 1
	// > < >= <= only applies to number values.
	ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN              ComparisonOperator = 2
	ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS    ComparisonOperator = 3
	ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN           ComparisonOperator = 4
	ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS ComparisonOperator = 5
	// Contained within only applies to list values. Elements
	// of the list must be primitive (i.e. number, string or bool)
	ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN ComparisonOperator = 6
	// Starts with and ends with only apply to string values.
	ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH ComparisonOperator = 7
	ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH   ComparisonOperator = 8
	// Contains only applies to string values, and for now is strict equality.
	// If we support things like regex or case insensitive matches, they will
	// be separate operators.
	ComparisonOperator_COMPARISON_OPERATOR_CONTAINS ComparisonOperator = 9
	// Present is the only operator that doesn't require a comparison value.
	ComparisonOperator_COMPARISON_OPERATOR_PRESENT ComparisonOperator = 10
	// != only applies to number, string and bool values.
	ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS ComparisonOperator = 11
)

// Enum value maps for ComparisonOperator.
var (
	ComparisonOperator_name = map[int32]string{
		0:  "COMPARISON_OPERATOR_UNSPECIFIED",
		1:  "COMPARISON_OPERATOR_EQUALS",
		2:  "COMPARISON_OPERATOR_LESS_THAN",
		3:  "COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS",
		4:  "COMPARISON_OPERATOR_GREATER_THAN",
		5:  "COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS",
		6:  "COMPARISON_OPERATOR_CONTAINED_WITHIN",
		7:  "COMPARISON_OPERATOR_STARTS_WITH",
		8:  "COMPARISON_OPERATOR_ENDS_WITH",
		9:  "COMPARISON_OPERATOR_CONTAINS",
		10: "COMPARISON_OPERATOR_PRESENT",
		11: "COMPARISON_OPERATOR_NOT_EQUALS",
	}
	ComparisonOperator_value = map[string]int32{
		"COMPARISON_OPERATOR_UNSPECIFIED":            0,
		"COMPARISON_OPERATOR_EQUALS":                 1,
		"COMPARISON_OPERATOR_LESS_THAN":              2,
		"COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS":    3,
		"COMPARISON_OPERATOR_GREATER_THAN":           4,
		"COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS": 5,
		"COMPARISON_OPERATOR_CONTAINED_WITHIN":       6,
		"COMPARISON_OPERATOR_STARTS_WITH":            7,
		"COMPARISON_OPERATOR_ENDS_WITH":              8,
		"COMPARISON_OPERATOR_CONTAINS":               9,
		"COMPARISON_OPERATOR_PRESENT":                10,
		"COMPARISON_OPERATOR_NOT_EQUALS":             11,
	}
)

func (x ComparisonOperator) Enum() *ComparisonOperator {
	p := new(ComparisonOperator)
	*p = x
	return p
}

func (x ComparisonOperator) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ComparisonOperator) Descriptor() protoreflect.EnumDescriptor {
	return file_lekko_rules_v1beta3_rules_proto_enumTypes[0].Descriptor()
}

func (ComparisonOperator) Type() protoreflect.EnumType {
	return &file_lekko_rules_v1beta3_rules_proto_enumTypes[0]
}

func (x ComparisonOperator) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ComparisonOperator.Descriptor instead.
func (ComparisonOperator) EnumDescriptor() ([]byte, []int) {
	return file_lekko_rules_v1beta3_rules_proto_rawDescGZIP(), []int{0}
}

type LogicalOperator int32

const (
	LogicalOperator_LOGICAL_OPERATOR_UNSPECIFIED LogicalOperator = 0
	LogicalOperator_LOGICAL_OPERATOR_AND         LogicalOperator = 1
	LogicalOperator_LOGICAL_OPERATOR_OR          LogicalOperator = 2
)

// Enum value maps for LogicalOperator.
var (
	LogicalOperator_name = map[int32]string{
		0: "LOGICAL_OPERATOR_UNSPECIFIED",
		1: "LOGICAL_OPERATOR_AND",
		2: "LOGICAL_OPERATOR_OR",
	}
	LogicalOperator_value = map[string]int32{
		"LOGICAL_OPERATOR_UNSPECIFIED": 0,
		"LOGICAL_OPERATOR_AND":         1,
		"LOGICAL_OPERATOR_OR":          2,
	}
)

func (x LogicalOperator) Enum() *LogicalOperator {
	p := new(LogicalOperator)
	*p = x
	return p
}

func (x LogicalOperator) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (LogicalOperator) Descriptor() protoreflect.EnumDescriptor {
	return file_lekko_rules_v1beta3_rules_proto_enumTypes[1].Descriptor()
}

func (LogicalOperator) Type() protoreflect.EnumType {
	return &file_lekko_rules_v1beta3_rules_proto_enumTypes[1]
}

func (x LogicalOperator) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use LogicalOperator.Descriptor instead.
func (LogicalOperator) EnumDescriptor() ([]byte, []int) {
	return file_lekko_rules_v1beta3_rules_proto_rawDescGZIP(), []int{1}
}

// A Rule is a top level object that recursively defines an AST represented
// by ruleslang. A rule is always one of 4 things:
// 1. Atom -> This is a leaf node in the tree that returns true or false
// 2. Not -> This negates the result of the underlying Rule.
// 3. LogicalExpression -> This rule links at least two rules through an "and" or an "or".
// 4. BoolConst -> true or false. This will be used for higher level short-circuits.
// Parentheses and other logical constructs can all be represented by the correct
// construction of this rule tree.
//
// !(A && B && C) || D can be represented by LogExp ( Not ( LogExp ( Atom(A) && Atom(B) && Atom(C) )) || Atom(D))
type Rule struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Types that are assignable to Rule:
	//	*Rule_Atom
	//	*Rule_Not
	//	*Rule_LogicalExpression
	//	*Rule_BoolConst
	Rule isRule_Rule `protobuf_oneof:"rule"`
}

func (x *Rule) Reset() {
	*x = Rule{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Rule) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Rule) ProtoMessage() {}

func (x *Rule) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Rule.ProtoReflect.Descriptor instead.
func (*Rule) Descriptor() ([]byte, []int) {
	return file_lekko_rules_v1beta3_rules_proto_rawDescGZIP(), []int{0}
}

func (m *Rule) GetRule() isRule_Rule {
	if m != nil {
		return m.Rule
	}
	return nil
}

func (x *Rule) GetAtom() *Atom {
	if x, ok := x.GetRule().(*Rule_Atom); ok {
		return x.Atom
	}
	return nil
}

func (x *Rule) GetNot() *Rule {
	if x, ok := x.GetRule().(*Rule_Not); ok {
		return x.Not
	}
	return nil
}

func (x *Rule) GetLogicalExpression() *LogicalExpression {
	if x, ok := x.GetRule().(*Rule_LogicalExpression); ok {
		return x.LogicalExpression
	}
	return nil
}

func (x *Rule) GetBoolConst() bool {
	if x, ok := x.GetRule().(*Rule_BoolConst); ok {
		return x.BoolConst
	}
	return false
}

type isRule_Rule interface {
	isRule_Rule()
}

type Rule_Atom struct {
	Atom *Atom `protobuf:"bytes,1,opt,name=atom,proto3,oneof"`
}

type Rule_Not struct {
	Not *Rule `protobuf:"bytes,2,opt,name=not,proto3,oneof"`
}

type Rule_LogicalExpression struct {
	LogicalExpression *LogicalExpression `protobuf:"bytes,3,opt,name=logical_expression,json=logicalExpression,proto3,oneof"`
}

type Rule_BoolConst struct {
	BoolConst bool `protobuf:"varint,4,opt,name=bool_const,json=boolConst,proto3,oneof"`
}

func (*Rule_Atom) isRule_Rule() {}

func (*Rule_Not) isRule_Rule() {}

func (*Rule_LogicalExpression) isRule_Rule() {}

func (*Rule_BoolConst) isRule_Rule() {}

// LogicalExpression operator applies a logical operator like "and" or "or" to n rules.
// They are evaluated in the order expressed by the repeated field.
type LogicalExpression struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Rules           []*Rule         `protobuf:"bytes,1,rep,name=rules,proto3" json:"rules,omitempty"`
	LogicalOperator LogicalOperator `protobuf:"varint,3,opt,name=logical_operator,json=logicalOperator,proto3,enum=lekko.rules.v1beta3.LogicalOperator" json:"logical_operator,omitempty"`
}

func (x *LogicalExpression) Reset() {
	*x = LogicalExpression{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LogicalExpression) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LogicalExpression) ProtoMessage() {}

func (x *LogicalExpression) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LogicalExpression.ProtoReflect.Descriptor instead.
func (*LogicalExpression) Descriptor() ([]byte, []int) {
	return file_lekko_rules_v1beta3_rules_proto_rawDescGZIP(), []int{1}
}

func (x *LogicalExpression) GetRules() []*Rule {
	if x != nil {
		return x.Rules
	}
	return nil
}

func (x *LogicalExpression) GetLogicalOperator() LogicalOperator {
	if x != nil {
		return x.LogicalOperator
	}
	return LogicalOperator_LOGICAL_OPERATOR_UNSPECIFIED
}

// An atom is a fragment of ruleslang that can result in a true or false.
// An atom always has a comparison operator and a context key, and can optionally
// have a comparison value.
type Atom struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ContextKey string `protobuf:"bytes,1,opt,name=context_key,json=contextKey,proto3" json:"context_key,omitempty"`
	// For the "PRESENT" operator, the comparison value should be null.
	ComparisonValue *structpb.Value `protobuf:"bytes,2,opt,name=comparison_value,json=comparisonValue,proto3" json:"comparison_value,omitempty"`
	// For operators, context is on the left, comparison value on the right.
	ComparisonOperator ComparisonOperator `protobuf:"varint,3,opt,name=comparison_operator,json=comparisonOperator,proto3,enum=lekko.rules.v1beta3.ComparisonOperator" json:"comparison_operator,omitempty"`
}

func (x *Atom) Reset() {
	*x = Atom{}
	if protoimpl.UnsafeEnabled {
		mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Atom) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Atom) ProtoMessage() {}

func (x *Atom) ProtoReflect() protoreflect.Message {
	mi := &file_lekko_rules_v1beta3_rules_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Atom.ProtoReflect.Descriptor instead.
func (*Atom) Descriptor() ([]byte, []int) {
	return file_lekko_rules_v1beta3_rules_proto_rawDescGZIP(), []int{2}
}

func (x *Atom) GetContextKey() string {
	if x != nil {
		return x.ContextKey
	}
	return ""
}

func (x *Atom) GetComparisonValue() *structpb.Value {
	if x != nil {
		return x.ComparisonValue
	}
	return nil
}

func (x *Atom) GetComparisonOperator() ComparisonOperator {
	if x != nil {
		return x.ComparisonOperator
	}
	return ComparisonOperator_COMPARISON_OPERATOR_UNSPECIFIED
}

var File_lekko_rules_v1beta3_rules_proto protoreflect.FileDescriptor

var file_lekko_rules_v1beta3_rules_proto_rawDesc = []byte{
	0x0a, 0x1f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2f, 0x76, 0x31,
	0x62, 0x65, 0x74, 0x61, 0x33, 0x2f, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x13, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x1a, 0x1c, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x73, 0x74, 0x72, 0x75, 0x63, 0x74, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0xe8, 0x01, 0x0a, 0x04, 0x52, 0x75, 0x6c, 0x65, 0x12, 0x2f, 0x0a,
	0x04, 0x61, 0x74, 0x6f, 0x6d, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6c, 0x65,
	0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x33, 0x2e, 0x41, 0x74, 0x6f, 0x6d, 0x48, 0x00, 0x52, 0x04, 0x61, 0x74, 0x6f, 0x6d, 0x12, 0x2d,
	0x0a, 0x03, 0x6e, 0x6f, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6c, 0x65,
	0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61,
	0x33, 0x2e, 0x52, 0x75, 0x6c, 0x65, 0x48, 0x00, 0x52, 0x03, 0x6e, 0x6f, 0x74, 0x12, 0x57, 0x0a,
	0x12, 0x6c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x5f, 0x65, 0x78, 0x70, 0x72, 0x65, 0x73, 0x73,
	0x69, 0x6f, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x26, 0x2e, 0x6c, 0x65, 0x6b, 0x6b,
	0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x2e,
	0x4c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x45, 0x78, 0x70, 0x72, 0x65, 0x73, 0x73, 0x69, 0x6f,
	0x6e, 0x48, 0x00, 0x52, 0x11, 0x6c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x45, 0x78, 0x70, 0x72,
	0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1f, 0x0a, 0x0a, 0x62, 0x6f, 0x6f, 0x6c, 0x5f, 0x63,
	0x6f, 0x6e, 0x73, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x08, 0x48, 0x00, 0x52, 0x09, 0x62, 0x6f,
	0x6f, 0x6c, 0x43, 0x6f, 0x6e, 0x73, 0x74, 0x42, 0x06, 0x0a, 0x04, 0x72, 0x75, 0x6c, 0x65, 0x22,
	0x95, 0x01, 0x0a, 0x11, 0x4c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x45, 0x78, 0x70, 0x72, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x2f, 0x0a, 0x05, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x19, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c,
	0x65, 0x73, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x2e, 0x52, 0x75, 0x6c, 0x65, 0x52,
	0x05, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x12, 0x4f, 0x0a, 0x10, 0x6c, 0x6f, 0x67, 0x69, 0x63, 0x61,
	0x6c, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x24, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x2e, 0x4c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x4f, 0x70,
	0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x52, 0x0f, 0x6c, 0x6f, 0x67, 0x69, 0x63, 0x61, 0x6c, 0x4f,
	0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x22, 0xc4, 0x01, 0x0a, 0x04, 0x41, 0x74, 0x6f, 0x6d,
	0x12, 0x1f, 0x0a, 0x0b, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x5f, 0x6b, 0x65, 0x79, 0x18,
	0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x63, 0x6f, 0x6e, 0x74, 0x65, 0x78, 0x74, 0x4b, 0x65,
	0x79, 0x12, 0x41, 0x0a, 0x10, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x69, 0x73, 0x6f, 0x6e, 0x5f,
	0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x16, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x56, 0x61,
	0x6c, 0x75, 0x65, 0x52, 0x0f, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x69, 0x73, 0x6f, 0x6e, 0x56,
	0x61, 0x6c, 0x75, 0x65, 0x12, 0x58, 0x0a, 0x13, 0x63, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x69, 0x73,
	0x6f, 0x6e, 0x5f, 0x6f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x0e, 0x32, 0x27, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e,
	0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x2e, 0x43, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x69, 0x73,
	0x6f, 0x6e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x52, 0x12, 0x63, 0x6f, 0x6d, 0x70,
	0x61, 0x72, 0x69, 0x73, 0x6f, 0x6e, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x2a, 0xd8,
	0x03, 0x0a, 0x12, 0x43, 0x6f, 0x6d, 0x70, 0x61, 0x72, 0x69, 0x73, 0x6f, 0x6e, 0x4f, 0x70, 0x65,
	0x72, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x23, 0x0a, 0x1f, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49,
	0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x55, 0x4e, 0x53,
	0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x1e, 0x0a, 0x1a, 0x43, 0x4f,
	0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f,
	0x52, 0x5f, 0x45, 0x51, 0x55, 0x41, 0x4c, 0x53, 0x10, 0x01, 0x12, 0x21, 0x0a, 0x1d, 0x43, 0x4f,
	0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f,
	0x52, 0x5f, 0x4c, 0x45, 0x53, 0x53, 0x5f, 0x54, 0x48, 0x41, 0x4e, 0x10, 0x02, 0x12, 0x2b, 0x0a,
	0x27, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52,
	0x41, 0x54, 0x4f, 0x52, 0x5f, 0x4c, 0x45, 0x53, 0x53, 0x5f, 0x54, 0x48, 0x41, 0x4e, 0x5f, 0x4f,
	0x52, 0x5f, 0x45, 0x51, 0x55, 0x41, 0x4c, 0x53, 0x10, 0x03, 0x12, 0x24, 0x0a, 0x20, 0x43, 0x4f,
	0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f,
	0x52, 0x5f, 0x47, 0x52, 0x45, 0x41, 0x54, 0x45, 0x52, 0x5f, 0x54, 0x48, 0x41, 0x4e, 0x10, 0x04,
	0x12, 0x2e, 0x0a, 0x2a, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f,
	0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x47, 0x52, 0x45, 0x41, 0x54, 0x45, 0x52, 0x5f,
	0x54, 0x48, 0x41, 0x4e, 0x5f, 0x4f, 0x52, 0x5f, 0x45, 0x51, 0x55, 0x41, 0x4c, 0x53, 0x10, 0x05,
	0x12, 0x28, 0x0a, 0x24, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f,
	0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x54, 0x41, 0x49, 0x4e, 0x45,
	0x44, 0x5f, 0x57, 0x49, 0x54, 0x48, 0x49, 0x4e, 0x10, 0x06, 0x12, 0x23, 0x0a, 0x1f, 0x43, 0x4f,
	0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f,
	0x52, 0x5f, 0x53, 0x54, 0x41, 0x52, 0x54, 0x53, 0x5f, 0x57, 0x49, 0x54, 0x48, 0x10, 0x07, 0x12,
	0x21, 0x0a, 0x1d, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50,
	0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x45, 0x4e, 0x44, 0x53, 0x5f, 0x57, 0x49, 0x54, 0x48,
	0x10, 0x08, 0x12, 0x20, 0x0a, 0x1c, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53, 0x4f, 0x4e,
	0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x43, 0x4f, 0x4e, 0x54, 0x41, 0x49,
	0x4e, 0x53, 0x10, 0x09, 0x12, 0x1f, 0x0a, 0x1b, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49, 0x53,
	0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x50, 0x52, 0x45, 0x53,
	0x45, 0x4e, 0x54, 0x10, 0x0a, 0x12, 0x22, 0x0a, 0x1e, 0x43, 0x4f, 0x4d, 0x50, 0x41, 0x52, 0x49,
	0x53, 0x4f, 0x4e, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x4e, 0x4f, 0x54,
	0x5f, 0x45, 0x51, 0x55, 0x41, 0x4c, 0x53, 0x10, 0x0b, 0x2a, 0x66, 0x0a, 0x0f, 0x4c, 0x6f, 0x67,
	0x69, 0x63, 0x61, 0x6c, 0x4f, 0x70, 0x65, 0x72, 0x61, 0x74, 0x6f, 0x72, 0x12, 0x20, 0x0a, 0x1c,
	0x4c, 0x4f, 0x47, 0x49, 0x43, 0x41, 0x4c, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52,
	0x5f, 0x55, 0x4e, 0x53, 0x50, 0x45, 0x43, 0x49, 0x46, 0x49, 0x45, 0x44, 0x10, 0x00, 0x12, 0x18,
	0x0a, 0x14, 0x4c, 0x4f, 0x47, 0x49, 0x43, 0x41, 0x4c, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54,
	0x4f, 0x52, 0x5f, 0x41, 0x4e, 0x44, 0x10, 0x01, 0x12, 0x17, 0x0a, 0x13, 0x4c, 0x4f, 0x47, 0x49,
	0x43, 0x41, 0x4c, 0x5f, 0x4f, 0x50, 0x45, 0x52, 0x41, 0x54, 0x4f, 0x52, 0x5f, 0x4f, 0x52, 0x10,
	0x02, 0x42, 0xde, 0x01, 0x0a, 0x17, 0x63, 0x6f, 0x6d, 0x2e, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2e,
	0x72, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x42, 0x0a, 0x52,
	0x75, 0x6c, 0x65, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x49, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x64, 0x65, 0x76,
	0x2f, 0x63, 0x6c, 0x69, 0x2f, 0x70, 0x6b, 0x67, 0x2f, 0x67, 0x65, 0x6e, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x67, 0x6f, 0x2f, 0x6c, 0x65, 0x6b, 0x6b, 0x6f, 0x2f, 0x72, 0x75, 0x6c, 0x65,
	0x73, 0x2f, 0x76, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x3b, 0x72, 0x75, 0x6c, 0x65, 0x73, 0x76,
	0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0xa2, 0x02, 0x03, 0x4c, 0x52, 0x58, 0xaa, 0x02, 0x13, 0x4c,
	0x65, 0x6b, 0x6b, 0x6f, 0x2e, 0x52, 0x75, 0x6c, 0x65, 0x73, 0x2e, 0x56, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x33, 0xca, 0x02, 0x13, 0x4c, 0x65, 0x6b, 0x6b, 0x6f, 0x5c, 0x52, 0x75, 0x6c, 0x65, 0x73,
	0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0xe2, 0x02, 0x1f, 0x4c, 0x65, 0x6b, 0x6b, 0x6f,
	0x5c, 0x52, 0x75, 0x6c, 0x65, 0x73, 0x5c, 0x56, 0x31, 0x62, 0x65, 0x74, 0x61, 0x33, 0x5c, 0x47,
	0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x15, 0x4c, 0x65, 0x6b,
	0x6b, 0x6f, 0x3a, 0x3a, 0x52, 0x75, 0x6c, 0x65, 0x73, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x65, 0x74,
	0x61, 0x33, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_lekko_rules_v1beta3_rules_proto_rawDescOnce sync.Once
	file_lekko_rules_v1beta3_rules_proto_rawDescData = file_lekko_rules_v1beta3_rules_proto_rawDesc
)

func file_lekko_rules_v1beta3_rules_proto_rawDescGZIP() []byte {
	file_lekko_rules_v1beta3_rules_proto_rawDescOnce.Do(func() {
		file_lekko_rules_v1beta3_rules_proto_rawDescData = protoimpl.X.CompressGZIP(file_lekko_rules_v1beta3_rules_proto_rawDescData)
	})
	return file_lekko_rules_v1beta3_rules_proto_rawDescData
}

var file_lekko_rules_v1beta3_rules_proto_enumTypes = make([]protoimpl.EnumInfo, 2)
var file_lekko_rules_v1beta3_rules_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_lekko_rules_v1beta3_rules_proto_goTypes = []interface{}{
	(ComparisonOperator)(0),   // 0: lekko.rules.v1beta3.ComparisonOperator
	(LogicalOperator)(0),      // 1: lekko.rules.v1beta3.LogicalOperator
	(*Rule)(nil),              // 2: lekko.rules.v1beta3.Rule
	(*LogicalExpression)(nil), // 3: lekko.rules.v1beta3.LogicalExpression
	(*Atom)(nil),              // 4: lekko.rules.v1beta3.Atom
	(*structpb.Value)(nil),    // 5: google.protobuf.Value
}
var file_lekko_rules_v1beta3_rules_proto_depIdxs = []int32{
	4, // 0: lekko.rules.v1beta3.Rule.atom:type_name -> lekko.rules.v1beta3.Atom
	2, // 1: lekko.rules.v1beta3.Rule.not:type_name -> lekko.rules.v1beta3.Rule
	3, // 2: lekko.rules.v1beta3.Rule.logical_expression:type_name -> lekko.rules.v1beta3.LogicalExpression
	2, // 3: lekko.rules.v1beta3.LogicalExpression.rules:type_name -> lekko.rules.v1beta3.Rule
	1, // 4: lekko.rules.v1beta3.LogicalExpression.logical_operator:type_name -> lekko.rules.v1beta3.LogicalOperator
	5, // 5: lekko.rules.v1beta3.Atom.comparison_value:type_name -> google.protobuf.Value
	0, // 6: lekko.rules.v1beta3.Atom.comparison_operator:type_name -> lekko.rules.v1beta3.ComparisonOperator
	7, // [7:7] is the sub-list for method output_type
	7, // [7:7] is the sub-list for method input_type
	7, // [7:7] is the sub-list for extension type_name
	7, // [7:7] is the sub-list for extension extendee
	0, // [0:7] is the sub-list for field type_name
}

func init() { file_lekko_rules_v1beta3_rules_proto_init() }
func file_lekko_rules_v1beta3_rules_proto_init() {
	if File_lekko_rules_v1beta3_rules_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_lekko_rules_v1beta3_rules_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Rule); i {
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
		file_lekko_rules_v1beta3_rules_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LogicalExpression); i {
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
		file_lekko_rules_v1beta3_rules_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Atom); i {
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
	file_lekko_rules_v1beta3_rules_proto_msgTypes[0].OneofWrappers = []interface{}{
		(*Rule_Atom)(nil),
		(*Rule_Not)(nil),
		(*Rule_LogicalExpression)(nil),
		(*Rule_BoolConst)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_lekko_rules_v1beta3_rules_proto_rawDesc,
			NumEnums:      2,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_lekko_rules_v1beta3_rules_proto_goTypes,
		DependencyIndexes: file_lekko_rules_v1beta3_rules_proto_depIdxs,
		EnumInfos:         file_lekko_rules_v1beta3_rules_proto_enumTypes,
		MessageInfos:      file_lekko_rules_v1beta3_rules_proto_msgTypes,
	}.Build()
	File_lekko_rules_v1beta3_rules_proto = out.File
	file_lekko_rules_v1beta3_rules_proto_rawDesc = nil
	file_lekko_rules_v1beta3_rules_proto_goTypes = nil
	file_lekko_rules_v1beta3_rules_proto_depIdxs = nil
}

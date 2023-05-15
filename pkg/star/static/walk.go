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

package static

import (
	"fmt"
	"strconv"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/rules/pkg/parser"
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

var (
	ErrUnsupportedStaticParsing = errors.New("unsupported static parsing")
)

// Walker provides methods to statically read and manipulate .star files
// that hold lekko features.
type Walker interface {
	Build() (*featurev1beta1.StaticFeature, error)
	// Note: this method will perform mutations based on the V3 ruleslang
	// AST if it exists. If not, it will fall back to the ruleString
	// provided in the feature.
	Mutate(f *featurev1beta1.StaticFeature) ([]byte, error)
	// Returns the formatted bytes.
	Format() ([]byte, error)
}

type walker struct {
	filename  string
	starBytes []byte
	registry  *protoregistry.Types
}

func NewWalker(filename string, starBytes []byte, registry *protoregistry.Types) *walker {
	return &walker{
		filename:  filename,
		starBytes: starBytes,
		registry:  registry,
	}
}

func (w *walker) Build() (*featurev1beta1.StaticFeature, error) {
	ast, err := w.genAST()
	if err != nil {
		return nil, errors.Wrap(err, "gen ast")
	}
	ret := &featurev1beta1.StaticFeature{
		Feature: &featurev1beta1.Feature{
			Tree: &featurev1beta1.Tree{},
		},
	}
	t := newTraverser(ast).
		withDefaultFn(w.buildDefaultFn(ret)).
		withDescriptionFn(w.buildDescriptionFn(ret)).
		withRulesFn(w.buildRulesFn(ret)).
		withProtoImportsFn(w.buildProtoImportsFn(ret))

	if err := t.traverse(); err != nil {
		return nil, errors.Wrap(err, "traverse")
	}
	return ret, nil
}

func (w *walker) Mutate(f *featurev1beta1.StaticFeature) ([]byte, error) {
	ast, err := w.genAST()
	if err != nil {
		return nil, errors.Wrap(err, "gen ast")
	}
	t := newTraverser(ast).
		withDefaultFn(w.mutateDefaultFn(f)).
		withDescriptionFn(w.mutateDescriptionFn(f)).
		withRulesFn(w.mutateRulesFn(f))

	if err := t.traverse(); err != nil {
		return nil, errors.Wrap(err, "traverse")
	}
	return t.format(), nil
}

func (w *walker) Format() ([]byte, error) {
	f, err := w.Build()
	if err != nil {
		return nil, errors.Wrap(err, "build")
	}
	return w.Mutate(f)
}

func (w *walker) genAST() (*build.File, error) {
	parser := butils.GetParser(InputTypeAuto)
	file, err := parser(w.filename, w.starBytes)
	if err != nil {
		return nil, errors.Wrap(err, "parse")
	}

	return file, nil
}

/** Methods helpful for building a proto-native representation of a feature **/

func (w *walker) extractValue(vPtr *build.Expr) (proto.Message, featurev1beta1.FeatureType, error) {
	if vPtr == nil {
		return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, fmt.Errorf("received nil value")
	}
	v := *vPtr
	switch t := v.(type) {
	case *build.Ident:
		switch t.Name {
		case "True":
			return wrapperspb.Bool(true), featurev1beta1.FeatureType_FEATURE_TYPE_BOOL, nil
		case "False":
			return wrapperspb.Bool(false), featurev1beta1.FeatureType_FEATURE_TYPE_BOOL, nil
		default:
			return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "unknown identifier %s", t.Name)
		}
	case *build.LiteralExpr:
		if strings.Contains(t.Token, ".") {
			if f, err := strconv.ParseFloat(t.Token, 64); err == nil {
				return wrapperspb.Double(f), featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT, nil
			}
		} else {
			if i, err := strconv.ParseInt(t.Token, 10, 64); err == nil {
				return wrapperspb.Int64(i), featurev1beta1.FeatureType_FEATURE_TYPE_INT, nil
			}
		}
	case *build.StringExpr:
		return wrapperspb.String(t.Value), featurev1beta1.FeatureType_FEATURE_TYPE_STRING, nil
	case *build.ListExpr: // json
		ret, err := w.extractJSONValue(v)
		return ret, featurev1beta1.FeatureType_FEATURE_TYPE_JSON, err
	case *build.DictExpr: // json
		ret, err := w.extractJSONValue(v)
		return ret, featurev1beta1.FeatureType_FEATURE_TYPE_JSON, err
	case *build.CallExpr:
		// try to statically parse the protobuf
		proto, err := CallExprToProto(t, w.registry)
		if err != nil {
			return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "unknown expression: %e", err)
		}
		return proto, featurev1beta1.FeatureType_FEATURE_TYPE_PROTO, nil
	}
	return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "type %T", v)
}

func (w *walker) extractJSONValue(v build.Expr) (*structpb.Value, error) {
	switch t := v.(type) {
	case *build.Ident:
		switch t.Name {
		case "True":
			return structpb.NewBoolValue(true), nil
		case "False":
			return structpb.NewBoolValue(false), nil
		case "None":
			return structpb.NewNullValue(), nil
		default:
			return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "unknown identifier %s", t.Name)
		}
	case *build.LiteralExpr:
		if f, err := strconv.ParseFloat(t.Token, 64); err == nil {
			return structpb.NewNumberValue(f), nil
		}
	case *build.StringExpr:
		return structpb.NewStringValue(t.Value), nil
	case *build.ListExpr:
		listVal := &structpb.ListValue{}
		for _, expr := range t.List {
			expr := expr
			elemVal, err := w.extractJSONValue(expr)
			if err != nil {
				return nil, errors.Wrap(err, "extract list elem value")
			}
			listVal.Values = append(listVal.Values, elemVal)
		}
		return structpb.NewListValue(listVal), nil
	case *build.DictExpr:
		structVal := structpb.Struct{
			Fields: map[string]*structpb.Value{},
		}
		for _, kvExpr := range t.List {
			kvExpr := kvExpr
			keyExpr, ok := kvExpr.Key.(*build.StringExpr)
			if !ok {
				return nil, errors.Errorf("json structs must have keys of type string, not %T", kvExpr.Key)
			}
			key := keyExpr.Value
			vVar, err := w.extractJSONValue(kvExpr.Value)
			if err != nil {
				return nil, errors.Wrap(err, "extract struct elem value")
			}
			structVal.Fields[key] = vVar
		}
		return structpb.NewStructValue(&structVal), nil
	}
	return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "type %T", v)
}

func (w *walker) buildDescriptionFn(f *featurev1beta1.StaticFeature) descriptionFn {
	return func(v *build.StringExpr) error {
		f.Description = v.Value
		f.Feature.Description = v.Value
		return nil
	}
}

func (w *walker) buildRulesFn(f *featurev1beta1.StaticFeature) rulesFn {
	return func(rulesW *rulesWrapper) error {
		for i, r := range rulesW.rules {
			rulesLang := r.conditionV.Value
			ast, err := parser.BuildAST(rulesLang)
			if err != nil {
				return errors.Wrapf(err, "build ast for rule '%s'", rulesLang)
			}
			astNew, err := parser.BuildASTV3(rulesLang)
			if err != nil {
				return errors.Wrapf(err, "build ast for rule '%s'", rulesLang)
			}
			rule := &featurev1beta1.Constraint{
				Rule:       rulesLang,
				RuleAst:    ast,
				RuleAstNew: astNew,
			}
			protoVal, featureType, err := w.extractValue(&r.v)
			if err != nil {
				return fmt.Errorf("rule #%d: extract value: %w", i, err)
			}
			ruleVal, err := w.toAny(protoVal)
			if err != nil {
				return errors.Wrapf(err, "extracted proto val to any for feature type %v", featureType)
			}
			rule.Value = ruleVal
			f.Feature.Tree.Constraints = append(f.Feature.Tree.Constraints, rule)
		}
		return nil
	}
}

func (w *walker) buildProtoImportsFn(f *featurev1beta1.StaticFeature) importsFn {
	return func(imports *importsWrapper) error {
		if imports == nil {
			return nil
		}
		typeError := func(expected, actual build.Expr) error {
			return errors.Errorf("expecting %T, found %T. expr: %s", expected, actual, build.FormatString(actual))
		}
		for _, starImport := range imports.imports {
			lhs, ok := starImport.assignExpr.LHS.(*build.Ident)
			if !ok {
				return typeError(&build.Ident{}, starImport.assignExpr.LHS)
			}
			rhs, ok := starImport.assignExpr.RHS.(*build.CallExpr)
			if !ok {
				return typeError(&build.CallExpr{}, starImport.assignExpr.RHS)
			}
			dotExpr, ok := rhs.X.(*build.DotExpr)
			if !ok {
				return typeError(&build.Ident{}, rhs.X)
			}
			dotX, ok := dotExpr.X.(*build.Ident)
			if !ok {
				return typeError(&build.Ident{}, dotExpr.X)
			}
			if dotX.Name != "proto" || dotExpr.Name != "package" {
				return errors.Errorf("found badly formed proto import: %s", build.FormatString(dotExpr))
			}
			var args []string
			for _, arg := range rhs.List {
				stringExpr, ok := arg.(*build.StringExpr)
				if !ok {
					return errors.Errorf("expecting string expr, found %T. expr: %s", arg, build.FormatString(arg))
				}
				args = append(args, stringExpr.Value)
			}
			f.Imports = append(f.Imports, &featurev1beta1.ImportStatement{
				Comments: commentBlockToProto(&starImport.assignExpr.Comments),
				Lhs: &featurev1beta1.IdentExpr{
					Comments: commentBlockToProto(&lhs.Comments),
					Token:    lhs.Name,
				},
				Operator:  starImport.assignExpr.Op,
				LineBreak: starImport.assignExpr.LineBreak,
				Rhs: &featurev1beta1.ImportExpr{
					Comments: commentBlockToProto(&rhs.Comments),
					Dot: &featurev1beta1.DotExpr{
						Comments: commentBlockToProto(&dotExpr.Comments),
						X:        dotX.Name,
						Name:     dotExpr.Name,
					},
					Args: args,
				},
			})
		}
		return nil
	}
}

func (w *walker) buildDefaultFn(f *featurev1beta1.StaticFeature) defaultFn {
	return func(v *build.Expr) error {
		protoVal, fType, err := w.extractValue(v)
		if err != nil {
			return err
		}
		anyVal, err := w.toAny(protoVal)
		if err != nil {
			return err
		}
		f.Feature.Tree.Default = anyVal
		f.Feature.Type = fType
		f.Type = fType
		return nil
	}
}

/** Methods helpful for mutating the underlying AST. **/
// TODO: this method should take a feature type and switch on the feature type.
// And then cast the proto val to that wrapper type.
func (w *walker) genValue(a *anypb.Any, sf *featurev1beta1.StaticFeature) (build.Expr, error) {
	switch sf.Feature.Type {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		bv := &wrapperspb.BoolValue{}
		if err := a.UnmarshalTo(bv); err != nil {
			return nil, err
		}
		return starBool(bv.Value), nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		iv := &wrapperspb.Int64Value{}
		if err := a.UnmarshalTo(iv); err != nil {
			return nil, err
		}
		return starInt(iv.Value), nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		dv := &wrapperspb.DoubleValue{}
		if err := a.UnmarshalTo(dv); err != nil {
			return nil, err
		}
		return starFloat(dv.Value), nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		sv := &wrapperspb.StringValue{}
		if err := a.UnmarshalTo(sv); err != nil {
			return nil, err
		}
		return starString(sv.Value), nil
	case featurev1beta1.FeatureType_FEATURE_TYPE_JSON:
		sv := &structpb.Value{}
		if err := a.UnmarshalTo(sv); err != nil {
			return nil, err
		}
		return w.genJSONValue(sv)
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		protoVal, err := w.fromAnyDynamic(a)
		if err != nil {
			return nil, err
		}
		imp, err := findImport(sf.Imports, protoVal)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find import that proto msg belongs to")
		}
		return ProtoToStatic(imp, protoVal)
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "proto val type %v", a.TypeUrl)
	}
}

func (w *walker) genJSONValue(val *structpb.Value) (build.Expr, error) {
	switch k := val.Kind.(type) {
	case *structpb.Value_NullValue:
		return &build.Ident{
			Name: "None",
		}, nil
	case *structpb.Value_BoolValue:
		return starBool(k.BoolValue), nil
	case *structpb.Value_StringValue:
		return starString(k.StringValue), nil
	case *structpb.Value_NumberValue:
		return starFloat(k.NumberValue), nil
	case *structpb.Value_ListValue:
		listExpr := &build.ListExpr{
			ForceMultiLine: true,
		}
		for _, listElem := range k.ListValue.GetValues() {
			expr, err := w.genJSONValue(listElem)
			if err != nil {
				return nil, errors.Wrap(err, "gen value list elem")
			}
			listExpr.List = append(listExpr.List, expr)
		}
		return listExpr, nil
	case *structpb.Value_StructValue:
		dictExpr := &build.DictExpr{
			ForceMultiLine: true,
		}
		for key, value := range k.StructValue.Fields {
			valExpr, err := w.genJSONValue(value)
			if err != nil {
				return nil, errors.Wrap(err, "gen value dict elem")
			}
			dictExpr.List = append(dictExpr.List, &build.KeyValueExpr{
				Key:   starString(key),
				Value: valExpr,
			})
		}
		return dictExpr, nil
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "structpb val type %T", k)
	}
}

func (w *walker) mutateDefaultFn(f *featurev1beta1.StaticFeature) defaultFn {
	return func(v *build.Expr) error {
		_, featureType, err := w.extractValue(v)
		if err != nil {
			return errors.Wrap(err, "extract default value")
		}
		if featureType != f.Type {
			return errors.Wrapf(err, "cannot mutate star feature type %v with arg feature type %v", featureType, f.Type)
		}
		gen, err := w.genValue(f.Feature.Tree.Default, f)
		if err != nil {
			return errors.Wrap(err, "gen value")
		}
		*v = gen
		return nil
	}
}

func (w *walker) mutateDescriptionFn(f *featurev1beta1.StaticFeature) descriptionFn {
	return func(v *build.StringExpr) error {
		v.Value = f.Feature.Description
		return nil
	}
}

func (w *walker) mutateRulesFn(f *featurev1beta1.StaticFeature) rulesFn {
	return func(rulesW *rulesWrapper) error {
		var newRules []rule
		for i, r := range f.Feature.Tree.GetConstraints() {
			gen, err := w.genValue(r.Value, f)
			if err != nil {
				return errors.Wrapf(err, "rule %d: gen value", i)
			}
			newRuleString := r.Rule
			if r.RuleAstNew != nil {
				newRuleString, err = parser.RuleToString(r.RuleAstNew)
				if err != nil {
					return errors.Wrap(err, "error attempting to parse v3 rule ast to string")
				}
			}
			newRule := rule{
				conditionV: &build.StringExpr{
					Value: newRuleString,
				},
				v: gen,
			}
			newRules = append(newRules, newRule)
		}
		rulesW.rules = newRules
		return nil
	}
}

// returns a dynamic proto message. Use when the contained type is
// not known at compile time. For known types, use a.UnmarshalTo(dest).
func (w *walker) fromAnyDynamic(a *anypb.Any) (proto.Message, error) {
	protoVal, err := anypb.UnmarshalNew(a, proto.UnmarshalOptions{
		Resolver: w.registry,
	})
	if err != nil {
		return nil, errors.Wrap(err, "any unmarshal")
	}
	return protoVal, nil
}

func (w *walker) toAny(pm proto.Message) (*anypb.Any, error) {
	ret := new(anypb.Any)
	if err := anypb.MarshalFrom(ret, pm, proto.MarshalOptions{Deterministic: true}); err != nil {
		return nil, err
	}
	return ret, nil
}

func starBool(b bool) build.Expr {
	identExpr := &build.Ident{}
	identExpr.Name = "False"
	if b {
		identExpr.Name = "True"
	}
	return identExpr
}

func starInt(i int64) build.Expr {
	return &build.LiteralExpr{
		Token: strconv.FormatInt(i, 10),
	}
}

func starFloat(f float64) build.Expr {
	return &build.LiteralExpr{
		Token: starlark.Float(f).String(),
	}
}

func starString(s string) build.Expr {
	return &build.StringExpr{
		Value: s,
	}
}

func commentBlockToProto(comments *build.Comments) *featurev1beta1.Comments {
	return &featurev1beta1.Comments{
		Before: commentsToProto(comments.Before),
		Suffix: commentsToProto(comments.Suffix),
		After:  commentsToProto(comments.After),
	}
}

func commentsToProto(comments []build.Comment) []*featurev1beta1.Comment {
	var ret []*featurev1beta1.Comment
	for _, c := range comments {
		ret = append(ret, commentToProto(c))
	}
	return ret
}

func commentToProto(comment build.Comment) *featurev1beta1.Comment {
	return &featurev1beta1.Comment{
		Token: comment.Token,
	}
}

// Given the list of proto imports that were defined by the starlark,
// find the specific import that declared the protobuf package that
// contains the schema for the provided message. Note: the message may be
// dynamic.
func findImport(imports []*featurev1beta1.ImportStatement, msg proto.Message) (*featurev1beta1.ImportStatement, error) {
	msgFullName := string(msg.ProtoReflect().Descriptor().FullName())
	for _, imp := range imports {
		if len(imp.Rhs.Args) == 0 {
			return nil, errors.Errorf("import statement found with no args: %v", imp.Rhs.String())
		}
		packagePrefix := imp.Rhs.Args[0]
		if strings.HasPrefix(msgFullName, packagePrefix) {
			return imp, nil
		}
	}
	return nil, errors.New("no proto import statements found")
}

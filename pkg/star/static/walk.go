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
	"sort"
	"strconv"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/lekkodev/cli/pkg/feature"
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
	// Note: static mutation is supported for all lekko types with the following
	// caveat: Any feature that has advanced starlark such as helper functions,
	// list comprehension, etc will fail to be statically parsed at the moment.
	Mutate(f *featurev1beta1.StaticFeature) ([]byte, error)
	// Returns the formatted bytes.
	Format() ([]byte, error)
}

type walker struct {
	filename  string
	starBytes []byte
	registry  *protoregistry.Types
	nv        feature.NamespaceVersion
}

func NewWalker(filename string, starBytes []byte, registry *protoregistry.Types, nv feature.NamespaceVersion) *walker {
	return &walker{
		filename:  filename,
		starBytes: starBytes,
		registry:  registry,
		nv:        nv,
	}
}

func (w *walker) Build() (*featurev1beta1.StaticFeature, error) {
	ast, err := w.genAST()
	if err != nil {
		return nil, errors.Wrap(err, "gen ast")
	}
	ret := &featurev1beta1.StaticFeature{
		Feature: &featurev1beta1.FeatureStruct{
			Rules: &featurev1beta1.Rules{},
		},
		FeatureOld: &featurev1beta1.Feature{
			Tree: &featurev1beta1.Tree{},
		},
	}
	t := newTraverser(ast, w.nv).
		withDefaultFn(w.buildDefaultFn(ret)).
		withDescriptionFn(w.buildDescriptionFn(ret)).
		withOverridesFn(w.buildRulesFn(ret)).
		withProtoImportsFn(w.buildProtoImportsFn(ret)).
		withMetadataFn(w.buildMetadataFn(ret))

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
	t := newTraverser(ast, w.nv).
		withDefaultFn(w.mutateDefaultFn(f)).
		withDescriptionFn(w.mutateDescriptionFn(f)).
		withOverridesFn(w.mutateOverridesFn(f)).
		withMetadataFn(w.mutateMetadataFn(f))

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

func (w *walker) extractValue(vPtr *build.Expr, f *featurev1beta1.StaticFeature) (proto.Message, featurev1beta1.FeatureType, error) {
	if vPtr == nil {
		return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "received nil value")
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
		if strings.Contains(t.Token, ".") || strings.Contains(t.Token, "e") || strings.Contains(t.Token, "E") {
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
		return w.extractProtoValue(t, f)
	case *build.DotExpr:
		// could be a proto enum
		return w.extractProtoValue(t, f)
	case *build.UnaryExpr:
		if t.Op == "-" {
			wrapper, featureType, err := w.extractValue(&t.X, f)
			if err != nil {
				return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, err
			}
			if featureType == featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT {
				wrapperD, _ := wrapper.(*wrapperspb.DoubleValue)
				wrapperD.Value = -wrapperD.Value
				return wrapperD, featureType, nil
			} else if featureType == featurev1beta1.FeatureType_FEATURE_TYPE_INT {
				wrapperI, _ := wrapper.(*wrapperspb.Int64Value)
				wrapperI.Value = -wrapperI.Value
				return wrapperI, featureType, nil
			} else {
				return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "unsupported negative on type %T", wrapper)
			}
		}
	}
	return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "%v type %T", v, v)
}

func (w *walker) extractProtoValue(v build.Expr, f *featurev1beta1.StaticFeature) (proto.Message, featurev1beta1.FeatureType, error) {
	proto, err := ExprToProto(v, f, w.registry)
	if err != nil {
		return nil, featurev1beta1.FeatureType_FEATURE_TYPE_UNSPECIFIED, errors.Wrapf(ErrUnsupportedStaticParsing, "unknown expression '%s': %e", build.FormatString(v), err)
	}
	return proto, featurev1beta1.FeatureType_FEATURE_TYPE_PROTO, nil
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
		structVal, err := w.extractJSONStruct(t)
		if err != nil {
			return nil, err
		}
		return structpb.NewStructValue(structVal), nil
	}
	return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "type %T", v)
}

func (w *walker) extractJSONStruct(d *build.DictExpr) (*structpb.Struct, error) {
	structVal := structpb.Struct{
		Fields: map[string]*structpb.Value{},
	}
	for _, kvExpr := range d.List {
		kvExpr := kvExpr
		keyExpr, ok := kvExpr.Key.(*build.StringExpr)
		if !ok {
			return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "json structs must have keys of type string, not %T", kvExpr.Key)
		}
		key := keyExpr.Value
		vVar, err := w.extractJSONValue(kvExpr.Value)
		if err != nil {
			return nil, errors.Wrap(err, "extract struct elem value")
		}
		structVal.Fields[key] = vVar
	}
	return &structVal, nil
}

func (w *walker) buildDescriptionFn(f *featurev1beta1.StaticFeature) descriptionFn {
	return func(v *build.StringExpr) error {
		f.Feature.Description = v.Value
		f.FeatureOld.Description = v.Value
		return nil
	}
}

func (w *walker) buildMetadataFn(f *featurev1beta1.StaticFeature) metadataFn {
	return func(ast *starFeatureAST) error {
		metadataExprPtr, found := ast.get(MetadataAttrName)
		if !found {
			return nil
		}
		metadataExpr := *metadataExprPtr
		metadataDict, ok := metadataExpr.(*build.DictExpr)
		if !ok {
			return errors.Wrapf(ErrUnsupportedStaticParsing, "metadata kwarg: expected dict, got %T", metadataExpr)
		}
		metadataStruct, err := w.extractJSONStruct(metadataDict)
		if err != nil {
			return errors.Wrap(err, "extract metadata")
		}
		f.Feature.Metadata = metadataStruct
		f.FeatureOld.Metadata = metadataStruct
		return nil
	}
}

func (w *walker) buildRulesFn(f *featurev1beta1.StaticFeature) overridesFn {
	return func(overridesW *overridesWrapper) error {
		for i, o := range overridesW.overrides {
			rulesLang := o.ruleV.Value
			astNew, err := parser.BuildASTV3(rulesLang)
			if err != nil {
				return errors.Wrapf(err, "build ast for rule '%s'", rulesLang)
			}
			override := &featurev1beta1.Constraint{
				Rule:       rulesLang,
				RuleAstNew: astNew,
			}
			protoVal, featureType, err := w.extractValue(&o.v, f) // #nosec G601
			if err != nil {
				return errors.Wrapf(err, "rule #%d: extract value", i)
			}
			overrideVal, err := w.toAny(protoVal)
			if err != nil {
				return errors.Wrapf(err, "extracted proto val to any for config type %v", featureType)
			}
			override.Value = overrideVal
			f.FeatureOld.Tree.Constraints = append(f.FeatureOld.Tree.Constraints, override)
			f.Feature.Rules.Rules = append(f.Feature.Rules.Rules, &featurev1beta1.Rule{
				Condition: rulesLang,
				Value: &featurev1beta1.StarExpr{
					Meta:       buildMeta(o.v),
					Expression: build.FormatString(o.v),
				},
			})
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
			return errors.Wrapf(ErrUnsupportedStaticParsing, "expecting %T, found %T. expr: %s", expected, actual, build.FormatString(actual))
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
				return errors.Wrapf(ErrUnsupportedStaticParsing, "found badly formed proto import: %s", build.FormatString(dotExpr))
			}
			var args []string
			for _, arg := range rhs.List {
				stringExpr, ok := arg.(*build.StringExpr)
				if !ok {
					return errors.Wrapf(ErrUnsupportedStaticParsing, "expecting string expr, found %T. expr: %s", arg, build.FormatString(arg))
				}
				args = append(args, stringExpr.Value)
			}
			f.Imports = append(f.Imports, &featurev1beta1.ImportStatement{
				Meta: buildMeta(starImport.assignExpr),
				Lhs: &featurev1beta1.IdentExpr{
					Meta:  buildMeta(lhs),
					Token: lhs.Name,
				},
				Operator:  starImport.assignExpr.Op,
				LineBreak: starImport.assignExpr.LineBreak,
				Rhs: &featurev1beta1.ImportExpr{
					Meta: buildMeta(rhs),
					Dot: &featurev1beta1.DotExpr{
						Meta: buildMeta(dotExpr),
						X:    dotX.Name,
						Name: dotExpr.Name,
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
		protoVal, fType, err := w.extractValue(v, f)
		if err != nil {
			return err
		}
		anyVal, err := w.toAny(protoVal)
		if err != nil {
			return err
		}
		f.FeatureOld.Tree.Default = anyVal
		f.FeatureOld.Type = fType
		f.Type = fType
		f.Feature.Default = &featurev1beta1.StarExpr{
			Meta:       buildMeta(*v),
			Expression: build.FormatString(*v),
		}
		return nil
	}
}

/** Methods helpful for mutating the underlying AST. **/
// TODO: this method should take a feature type and switch on the feature type.
// And then cast the proto val to that wrapper type.
func (w *walker) genValue(a *anypb.Any, sf *featurev1beta1.StaticFeature, meta *featurev1beta1.StarMeta) (build.Expr, error) {
	switch sf.FeatureOld.Type {
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
		return w.genJSONValue(sv, meta)
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		protoVal, err := w.fromAnyDynamic(a)
		if err != nil {
			return nil, err
		}
		// TODO: force sub multiline as well
		callExpr, err := ProtoToStatic(sf.GetImports(), protoVal.ProtoReflect(), meta)
		if err != nil {
			return nil, err
		}
		return callExpr, nil
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "proto val type %v", a.TypeUrl)
	}
}

func (w *walker) genJSONStruct(s *structpb.Struct, meta *featurev1beta1.StarMeta) (*build.DictExpr, error) {
	dictExpr := &build.DictExpr{
		ForceMultiLine: true,
	}
	for key, value := range s.Fields {
		valExpr, err := w.genJSONValue(value, meta)
		if err != nil {
			return nil, errors.Wrap(err, "gen value dict elem")
		}
		dictExpr.List = append(dictExpr.List, &build.KeyValueExpr{
			Key:   starString(key),
			Value: valExpr,
		})
	}
	sortKVs(dictExpr.List)
	return dictExpr, nil
}

func (w *walker) genJSONValue(val *structpb.Value, meta *featurev1beta1.StarMeta) (build.Expr, error) {
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
		intVal := int64(k.NumberValue)
		if k.NumberValue == float64(intVal) {
			return starInt(intVal), nil
		}
		return starFloat(k.NumberValue), nil
	case *structpb.Value_ListValue:
		listExpr := &build.ListExpr{
			ForceMultiLine: meta.GetMultiline(),
		}
		for _, listElem := range k.ListValue.GetValues() {
			expr, err := w.genJSONValue(listElem, meta)
			if err != nil {
				return nil, errors.Wrap(err, "gen value list elem")
			}
			listExpr.List = append(listExpr.List, expr)
		}
		return listExpr, nil
	case *structpb.Value_StructValue:
		dictExpr, err := w.genJSONStruct(k.StructValue, meta)
		if err != nil {
			return nil, err
		}
		return dictExpr, nil
	default:
		return nil, errors.Wrapf(ErrUnsupportedStaticParsing, "structpb val type %T", k)
	}
}

func sortKVs(l []*build.KeyValueExpr) {
	sort.Slice(l, func(i, j int) bool {
		return build.FormatString(l[i]) < build.FormatString(l[j])
	})
}

func (w *walker) mutateDefaultFn(f *featurev1beta1.StaticFeature) defaultFn {
	return func(v *build.Expr) error {
		_, featureType, err := w.extractValue(v, f)
		if err != nil {
			return errors.Wrap(err, "extract default value")
		}
		if featureType != f.Type {
			return errors.Wrapf(err, "cannot mutate star config type %v with arg config type %v", featureType, f.Type)
		}
		gen, err := w.genValue(f.FeatureOld.Tree.Default, f, f.GetFeature().GetDefault().GetMeta())
		if err != nil {
			return errors.Wrap(err, "gen value")
		}
		*v = gen
		return nil
	}
}

func (w *walker) mutateDescriptionFn(f *featurev1beta1.StaticFeature) descriptionFn {
	return func(v *build.StringExpr) error {
		v.Value = f.FeatureOld.Description
		return nil
	}
}

func (w *walker) mutateMetadataFn(f *featurev1beta1.StaticFeature) metadataFn {
	return func(ast *starFeatureAST) error {
		metadataProto := f.FeatureOld.GetMetadata()
		if metadataProto == nil {
			ast.unset(MetadataAttrName)
			return nil
		}
		metadataStarDict, err := w.genJSONStruct(metadataProto, nil)
		if err != nil {
			return err
		}
		ast.set(MetadataAttrName, metadataStarDict)
		return nil
	}
}

func (w *walker) mutateOverridesFn(f *featurev1beta1.StaticFeature) overridesFn {
	return func(overridesW *overridesWrapper) error {
		var newOverrides []override
		for i, r := range f.FeatureOld.Tree.GetConstraints() {
			var meta *featurev1beta1.StarMeta
			rules := f.GetFeature().GetRules().GetRules()
			if len(rules) > i {
				meta = rules[i].GetValue().GetMeta()
			}
			if r.Value == nil {
				r.Value = &anypb.Any{
					TypeUrl: r.GetValueNew().GetTypeUrl(),
					Value:   r.GetValueNew().GetValue(),
				}
			}
			gen, err := w.genValue(r.Value, f, meta)
			if err != nil {
				return errors.Wrapf(err, "override %d: gen value", i)
			}
			newRuleString := r.Rule
			if r.RuleAstNew != nil {
				newRuleString, err = parser.RuleToString(r.RuleAstNew)
				if err != nil {
					return errors.Wrap(err, "error attempting to parse v3 rule ast to string")
				}
			}
			newOverride := override{
				ruleV: &build.StringExpr{
					Value: newRuleString,
				},
				v: gen,
			}
			newOverrides = append(newOverrides, newOverride)
		}
		overridesW.overrides = newOverrides
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

func buildMeta(expr build.Expr) *featurev1beta1.StarMeta {
	ret := &featurev1beta1.StarMeta{
		Comments: commentBlockToProto(expr.Comment()),
	}
	switch t := expr.(type) {
	case *build.CallExpr:
		ret.Multiline = t.ForceMultiLine || t.ListStart.Line < t.End.Pos.Line
	case *build.ListExpr:
		ret.Multiline = t.ForceMultiLine
	case *build.DictExpr:
		ret.Multiline = t.ForceMultiLine
	}
	return ret
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

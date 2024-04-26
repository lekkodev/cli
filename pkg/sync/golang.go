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

package sync

import (
	"bytes"
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"

	"log"
	"path"
	"strconv"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/assert"
	"github.com/lekkodev/cli/pkg/feature"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/star"
	"github.com/lekkodev/cli/pkg/star/static"
	"github.com/lekkodev/go-sdk/pkg/eval"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func Bisync(ctx context.Context, outputPath, lekkoPath, repoPath string) ([]string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, errors.Wrap(err, "find go.mod in working directory")
	}
	mf, err := modfile.ParseLax("go.mod", b, nil)
	if err != nil {
		return nil, err
	}

	// Traverse target path, finding namespaces
	// TODO: consider making this more efficient for batch gen/sync
	files := make([]string, 0)
	if err := filepath.WalkDir(lekkoPath, func(p string, d fs.DirEntry, err error) error {
		// Skip generated proto dir
		if d.IsDir() && d.Name() == "proto" {
			return filepath.SkipDir
		}
		// Sync and gen
		if d.Name() == "lekko.go" {
			if err := SyncGo(ctx, p, repoPath); err != nil {
				return errors.Wrapf(err, "sync %s", p)
			}
			namespace := filepath.Base(filepath.Dir(p))
			generator := gen.NewGoGenerator(mf.Module.Mod.Path, outputPath, lekkoPath, repoPath, namespace)
			if err := generator.Gen(ctx); err != nil {
				return errors.Wrapf(err, "generate code for %s", namespace)
			}
			files = append(files, p)
			fmt.Printf("Successfully bisynced %s\n", logging.Bold(p))
		}
		// Ignore others
		return nil
	}); err != nil {
		return nil, err
	}
	return files, nil
}

// TODO - make this our proto rep?
type Namespace struct {
	Name     string
	Features []*featurev1beta1.Feature
}

func SyncGo(ctx context.Context, f, repoPath string) error {
	src, err := os.ReadFile(f)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("open %s", f))
	}
	if bytes.Contains(src, []byte("<<<<<<<")) {
		return fmt.Errorf("%s has unresolved merge conflicts", f)
	}

	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, f, src, parser.ParseComments)
	if err != nil {
		return err
	}
	namespace := Namespace{}

	r, err := repo.NewLocal(repoPath, nil)
	if err != nil {
		return err
	}
	// Discard logs, mainly for silencing compilation later
	// TODO: Maybe a verbose flag
	r.ConfigureLogger(&repo.LoggingConfiguration{
		Writer: io.Discard,
	})
	rootMD, _, err := r.ParseMetadata(ctx)
	if err != nil {
		return err
	}
	registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
	if err != nil {
		return err
	}

	// TODO: instead of panicking everywhere, collect errors (maybe using go/analysis somehow)
	// so we can report them properly (and not look sketchy)
	ast.Inspect(pf, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			// i.e. lekkodefault -> default (this requires the package name to be correct)
			if !strings.HasPrefix(x.Name.Name, "lekko") {
				panic("packages for lekko must start with 'lekko'")
			}
			namespace.Name = x.Name.Name[5:]
			if len(namespace.Name) == 0 {
				panic("namespace name cannot be empty")
			}
		case *ast.FuncDecl:
			// TODO: We should support numbers (e.g. v2) but the strcase pkg has some non-ideal behavior with numbers,
			// we might want to write our own librar(ies) with cross-language consistency
			if regexp.MustCompile("^get[A-Z][A-Za-z]*$").MatchString(x.Name.Name) {
				var commentLines []string
				if x.Doc != nil {
					for _, comment := range x.Doc.List {
						commentLines = append(commentLines, strings.TrimLeft(comment.Text, "/ "))
					}
				}
				privateName := x.Name.Name
				configName := strcase.ToKebab(privateName[3:])
				results := x.Type.Results.List
				if results == nil {
					panic("must have a return type")
				}
				if len(results) != 1 {
					panic("must have one return type")
				}
				feature := &featurev1beta1.Feature{Key: configName, Description: strings.Join(commentLines, " "), Tree: &featurev1beta1.Tree{}}
				namespace.Features = append(namespace.Features, feature)
				switch t := results[0].Type.(type) {
				case *ast.Ident:
					switch t.Name {
					case "bool":
						feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_BOOL
					case "int64":
						feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_INT
					case "float64":
						feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT
					case "string":
						feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_STRING
					default:
						fmt.Printf("Unknown Ident: %+v\n", t)
					}
				case *ast.StarExpr:
					feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_PROTO
				default:
					fmt.Printf("%#v\n", t)
				}
				for _, stmt := range x.Body.List {
					switch n := stmt.(type) {
					case *ast.ReturnStmt:
						if feature.Tree.Default != nil {
							panic("Panic. Or Noop.")
						}
						// TODO also need to take care of the possibility that the default is in an else
						feature.Tree.Default = exprToAny(n.Results[0], registry, feature.Type) // can this be multiple things?
					case *ast.IfStmt:
						feature.Tree.Constraints = append(feature.Tree.Constraints, ifToConstraints(n, registry, feature.Type)...)
					default:
						panic("Panic!")
					}
				}
				return false
			}
			panic(fmt.Sprintf("sync %s: only functions like 'getConfig' are supported", x.Name.Name))
		}
		return true
	})
	// TODO static context

	nsExists := false
	// Need to keep track of which configs were synced
	// Any configs that were already present but not synced should be removed
	toRemove := make(map[string]struct{}) // Set of config names in existing namespace
	for _, nsFromMeta := range rootMD.Namespaces {
		if namespace.Name == nsFromMeta {
			nsExists = true
			ffs, err := r.GetFeatureFiles(ctx, namespace.Name)
			if err != nil {
				return errors.Wrap(err, "read existing configs")
			}
			for _, ff := range ffs {
				toRemove[ff.Name] = struct{}{}
			}
			break
		}
	}
	if !nsExists {
		if err := r.AddNamespace(ctx, namespace.Name); err != nil {
			return errors.Wrap(err, "add namespace")
		}
	}
	for _, configProto := range namespace.Features {
		// create a new starlark file from a template (based on the config type)
		var starBytes []byte
		starImports := make([]*featurev1beta1.ImportStatement, 0)

		if configProto.Type == featurev1beta1.FeatureType_FEATURE_TYPE_PROTO {
			typeURL := configProto.GetTree().GetDefault().GetTypeUrl()
			messageType, found := strings.CutPrefix(typeURL, "type.googleapis.com/")
			if !found {
				return fmt.Errorf("can't parse type url: %s", typeURL)
			}
			starInputs, err := r.BuildProtoStarInputs(ctx, messageType, feature.LatestNamespaceVersion())
			if err != nil {
				return err
			}
			starBytes, err = star.RenderExistingProtoTemplate(*starInputs, feature.LatestNamespaceVersion())
			if err != nil {
				return err
			}
			for importPackage, importAlias := range starInputs.Packages {
				starImports = append(starImports, &featurev1beta1.ImportStatement{
					Lhs: &featurev1beta1.IdentExpr{
						Token: importAlias,
					},
					Operator: "=",
					Rhs: &featurev1beta1.ImportExpr{
						Dot: &featurev1beta1.DotExpr{
							X:    "proto",
							Name: "package",
						},
						Args: []string{importPackage},
					},
				})
			}
		} else {
			starBytes, err = star.GetTemplate(eval.ConfigTypeFromProto(configProto.Type), feature.LatestNamespaceVersion(), nil)
			if err != nil {
				return err
			}
		}

		// mutate starlark with the actual config
		walker := static.NewWalker("", starBytes, registry, feature.NamespaceVersionV1Beta7)
		newBytes, err := walker.Mutate(&featurev1beta1.StaticFeature{
			Key:  configProto.Key,
			Type: configProto.GetType(),
			Feature: &featurev1beta1.FeatureStruct{
				Description: configProto.GetDescription(),
			},
			FeatureOld: configProto,
			Imports:    starImports,
		})
		if err != nil {
			return errors.Wrap(err, "walker mutate")
		}
		configFile := feature.NewFeatureFile(namespace.Name, configProto.Key)
		// write starlark to disk
		if err := r.WriteFile(path.Join(namespace.Name, configFile.StarlarkFileName), newBytes, 0600); err != nil {
			return errors.Wrap(err, "write after mutation")
		}

		// compile newly generated starlark file
		_, err = r.Compile(ctx, &repo.CompileRequest{
			Registry:        registry,
			NamespaceFilter: namespace.Name,
			FeatureFilter:   configProto.Key,
		})
		if err != nil {
			return errors.Wrap(err, "compile after mutation")
		}
		delete(toRemove, configProto.Key)
	}
	// Remove leftovers
	for configName := range toRemove {
		if err := r.RemoveFeature(ctx, namespace.Name, configName); err != nil {
			return errors.Wrapf(err, "remove %s", configName)
		}
	}

	return nil
}

func exprToValue(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	assert.Equal(ok, true, "value expr is not an identifier")
	return strcase.ToSnake(ident.Name)
}

// TODO -- We know the return type..
func exprToAny(expr ast.Expr, registry *protoregistry.Types, want featurev1beta1.FeatureType) *anypb.Any {
	switch node := expr.(type) {
	case *ast.BasicLit:
		switch node.Kind {
		case token.STRING:
			a, err := anypb.New(&wrapperspb.StringValue{Value: strings.Trim(node.Value, "\"`")})
			if err != nil {
				panic(err)
			}
			return a
		case token.INT:
			fallthrough
		case token.FLOAT:
			switch want {
			case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
				intValue, err := strconv.ParseInt(node.Value, 10, 64)
				if err != nil {
					panic(err)
				}
				a, err := anypb.New(&wrapperspb.Int64Value{Value: intValue})
				if err != nil {
					panic(err)
				}
				return a

			case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
				floatValue, err := strconv.ParseFloat(node.Value, 64)
				if err != nil {
					panic(err)
				}
				a, err := anypb.New(&wrapperspb.DoubleValue{Value: floatValue})
				if err != nil {
					panic(err)
				}
				return a
			}
		default:
			fmt.Printf("NV: %s\n", node.Value)
		}
	case *ast.Ident:
		switch node.Name {
		case "true":
			a, err := anypb.New(&wrapperspb.BoolValue{Value: true})
			if err != nil {
				panic(err)
			}
			return a
		case "false":
			a, err := anypb.New(&wrapperspb.BoolValue{Value: false})
			if err != nil {
				panic(err)
			}
			return a
		default:
			fmt.Printf("NN: %s\n", node.Name)
		}
	case *ast.UnaryExpr:
		switch node.Op {
		case token.AND:
			switch x := node.X.(type) {
			case *ast.CompositeLit:
				a, err := anypb.New(compositeLitToProto(x, registry).(protoreflect.ProtoMessage))
				if err != nil {
					panic(err)
				}
				return a
			default:
				panic("Unknown X Type")
			}
		default:
			panic("Unknown Op")
		}
	default:
		fmt.Printf("ETA: %#v\n", node)
	}
	return &anypb.Any{}
}

func exprToNameParts(expr ast.Expr) []string {
	switch node := expr.(type) {
	case *ast.Ident:
		return []string{node.Name}
	case *ast.SelectorExpr:
		return append(exprToNameParts(node.X), exprToNameParts(node.Sel)...)
	default:
		panic(node)
	}
}

func findMessageType(x *ast.CompositeLit, registry *protoregistry.Types) protoreflect.MessageType {
	innerIdent, ok := x.Type.(*ast.SelectorExpr).X.(*ast.Ident)
	if ok && innerIdent.Name == "durationpb" {
		mt, err := registry.FindMessageByName(protoreflect.FullName("google.protobuf").Append(protoreflect.Name(x.Type.(*ast.SelectorExpr).Sel.Name)))
		if err == nil {
			return mt
		}
		panic(err)
	}
	fullName := protoreflect.FullName("default.config.v1beta1")
	parts := exprToNameParts(x.Type)[1:]
	fullName = fullName.Append(protoreflect.Name(parts[0]))
	mt, err := registry.FindMessageByName(fullName)
	if err != nil {
		if strings.Contains(string(fullName), "_") {
			outerMessageDescriptor, err := registry.FindMessageByName(protoreflect.FullName(strings.Split(string(fullName), "_")[0]))
			if err == nil {
				for i := 0; i < outerMessageDescriptor.Descriptor().Messages().Len(); i = i + 1 {
					newMT := dynamicpb.NewMessageType(outerMessageDescriptor.Descriptor().Messages().Get(i))
					err := registry.RegisterMessage(newMT)
					assert.NoError(err, "register nested message")
					mt = newMT
				}
			}
		} else if mt == nil {
			log.Fatal("this strange bug above didn't catch this error", err)
		}
	}
	if len(parts) == 2 {
		md := mt.Descriptor().Messages().ByName(protoreflect.Name(parts[1])) // TODO
		panic(md)
	}
	return mt
}

func compositeLitToProto(x *ast.CompositeLit, registry *protoregistry.Types) protoreflect.Message {
	mt := findMessageType(x, registry)
	msg := mt.New()
	for _, v := range x.Elts {
		kv, ok := v.(*ast.KeyValueExpr)
		assert.Equal(ok, true)
		keyIdent, ok := kv.Key.(*ast.Ident)
		assert.Equal(ok, true)
		name := strcase.ToSnake(keyIdent.Name)
		field := mt.Descriptor().Fields().ByName(protoreflect.Name(name))
		if field == nil {
			continue // TODO...
		}
		switch node := kv.Value.(type) {
		case *ast.BasicLit:
			switch node.Kind {
			case token.STRING:
				msg.Set(field, protoreflect.ValueOf(strings.Trim(node.Value, "\"`")))
			case token.INT:
				if field.Kind() == protoreflect.EnumKind {
					intValue, err := strconv.ParseInt(node.Value, 10, 32)
					if err != nil {
						panic(errors.Wrapf(err, "int parse token %s", node.Value))
					}
					msg.Set(field, protoreflect.ValueOf(protoreflect.EnumNumber(intValue)))
					continue
				}
				// TODO - parse/validate based on field Kind
				if intValue, err := strconv.ParseInt(node.Value, 10, 64); err == nil {
					msg.Set(field, protoreflect.ValueOf(intValue))
				} else {
					panic(errors.Wrapf(err, "int parse token %s", node.Value))
				}
			case token.FLOAT:
				if floatValue, err := strconv.ParseFloat(node.Value, 64); err == nil {
					msg.Set(field, protoreflect.ValueOf(floatValue))
				} else {
					panic(errors.Wrapf(err, "float parse token %s", node.Value))
				}
			// Booleans are handled separately as literal identifiers below
			default:
				panic(fmt.Errorf("unsupported basic literal token type %v", node.Kind))
			}
		case *ast.Ident:
			switch node.Name {
			case "true":
				msg.Set(field, protoreflect.ValueOf(true))
			case "false":
				msg.Set(field, protoreflect.ValueOf(false))
			default:
				panic(fmt.Errorf("unsupported identifier %v", node.Name))
			}
		case *ast.UnaryExpr:
			switch node.Op {
			case token.AND:
				switch ix := node.X.(type) {
				case *ast.CompositeLit:
					msg.Set(field, protoreflect.ValueOf(compositeLitToProto(ix, registry)))
				default:
					panic(fmt.Errorf("unsupported X type for unary & %T", ix))
				}
			default:
				panic(fmt.Errorf("unsupported unary operator %v", node.Op))
			}
		case *ast.CompositeLit:
			switch clTypeNode := node.Type.(type) {
			case *ast.ArrayType:
				switch eltNode := clTypeNode.Elt.(type) {
				case *ast.Ident:
					// Primitive type array
					// TODO
				case *ast.StarExpr:
					// Proto type array
					// TODO
				default:
					panic(fmt.Errorf("unsupported slice element type %+v", eltNode))
				}
			case *ast.MapType:
				mapTypeNode := clTypeNode
				// TODO: Currently only supports primitive kvs
				switch mapTypeNode.Key.(type) {
				case *ast.Ident:
					// Do something
				default:
					panic(fmt.Errorf("unsupported map key type %+v", mapTypeNode))
				}
				switch mapTypeNode.Value.(type) {
				case *ast.Ident:
					// Do something
				default:
					panic(fmt.Errorf("unsupported map value type %+v", mapTypeNode))
				}
				for _, elt := range node.Elts {
					pair, ok := elt.(*ast.KeyValueExpr)
					assert.Equal(ok, true)
					basicLit, ok := pair.Key.(*ast.BasicLit)
					assert.Equal(ok, true)
					key := protoreflect.ValueOfString(strings.Trim(basicLit.Value, "\"")).MapKey()
					switch v := pair.Value.(type) {
					case *ast.BasicLit:
						switch v.Kind {
						case token.STRING:
							msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(strings.Trim(v.Value, "\"`")))
						case token.INT:
							if field.Kind() == protoreflect.EnumKind {
								intValue, err := strconv.ParseInt(v.Value, 10, 32)
								if err != nil {
									panic(err)
								}
								msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(protoreflect.EnumNumber(intValue)))
								continue
							}
							// TODO - parse/validate based on field Kind
							if intValue, err := strconv.ParseInt(v.Value, 10, 64); err == nil {
								msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(intValue))
							} else {
								panic(err)
							}
						case token.FLOAT:
							if floatValue, err := strconv.ParseFloat(v.Value, 64); err == nil {
								msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(floatValue))
							} else {
								panic(err)
							}
						default:
							fmt.Printf("NV: %s\n", v.Value)
						}
					case *ast.Ident:
						switch v.Name {
						case "true":
							msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(true))
						case "false":
							msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(false))
						default:
							fmt.Printf("NN: %s\n", v.Name)
						}
					}
				}
			default:
				panic(fmt.Errorf("unsupported composite literal type %T", clTypeNode))
			}
		default:
			panic(fmt.Errorf("unsupported composite literal value %+v", node))
		}
	}
	return msg
}

func exprToComparisonValue(expr ast.Expr) *structpb.Value {
	switch node := expr.(type) {
	case *ast.BasicLit:
		switch node.Kind {
		case token.STRING:
			return &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: strings.Trim(node.Value, "\"`"),
				},
			}
		case token.INT:
			intValue, err := strconv.ParseInt(node.Value, 10, 64)
			if err != nil {
				panic(err)
			}
			return &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: float64(intValue),
				},
			}

		case token.FLOAT:
			floatValue, err := strconv.ParseFloat(node.Value, 64)
			if err != nil {
				panic(err)
			}
			return &structpb.Value{
				Kind: &structpb.Value_NumberValue{
					NumberValue: floatValue,
				},
			}

		default:
			fmt.Printf("Unknown basicLit: %s\n", node.Value)
		}
	case *ast.Ident:
		switch node.Name {
		case "true":
			return &structpb.Value{
				Kind: &structpb.Value_BoolValue{
					BoolValue: true,
				},
			}
		case "false":
			return &structpb.Value{
				Kind: &structpb.Value_BoolValue{
					BoolValue: false,
				},
			}
		default:
			fmt.Printf("NN: %s\n", node.Name)
		}
	case *ast.CompositeLit:
		var list []*structpb.Value
		for _, elt := range node.Elts {
			list = append(list, exprToComparisonValue(elt))
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: list,
				},
			},
		}
	default:
		fmt.Printf("ETC: %s", node)
	}
	return &structpb.Value{}
}

func binaryExprToRule(expr *ast.BinaryExpr) *rulesv1beta3.Rule {
	switch expr.Op {
	case token.LAND:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND, Rules: []*rulesv1beta3.Rule{exprToRule(expr.X), exprToRule(expr.Y)}}}}
	case token.LOR:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR, Rules: []*rulesv1beta3.Rule{exprToRule(expr.X), exprToRule(expr.Y)}}}}
	case token.EQL:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	case token.LSS:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	case token.GTR:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	case token.NEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	case token.LEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	case token.GEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS, ContextKey: exprToValue(expr.X), ComparisonValue: exprToComparisonValue(expr.Y)}}}
	default:
		panic("Don't know how to toke")
	}
}

func callExprToRule(expr *ast.CallExpr) *rulesv1beta3.Rule {
	// TODO check Fun
	//fmt.Printf("\t%+v\n", expr.Fun)
	selectorExpr, ok := expr.Fun.(*ast.SelectorExpr)
	assert.Equal(ok, true)
	ident, ok := selectorExpr.X.(*ast.Ident)
	assert.Equal(ok, true)
	switch ident.Name { // TODO... brittle..
	case "slices":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ContextKey: exprToValue(expr.Args[1]), ComparisonValue: exprToComparisonValue(expr.Args[0])}}}
		default:
			panic("Ahhhh")
		}
	case "strings":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS, ContextKey: exprToValue(expr.Args[0]), ComparisonValue: exprToComparisonValue(expr.Args[1])}}}
		case "HasPrefix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH, ContextKey: exprToValue(expr.Args[0]), ComparisonValue: exprToComparisonValue(expr.Args[1])}}}
		case "HasSuffix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH, ContextKey: exprToValue(expr.Args[0]), ComparisonValue: exprToComparisonValue(expr.Args[1])}}}
		default:
			panic("Ahhhh")
		}
	default:
		panic("Ahhhh")
	}
}

func exprToRule(expr ast.Expr) *rulesv1beta3.Rule {
	switch node := expr.(type) {
	case *ast.BinaryExpr:
		return binaryExprToRule(node)
	case *ast.CallExpr:
		return callExprToRule(node)
	default:
		panic(fmt.Errorf("unsupported expression type for rule: %T", node))
	}
}

func ifToConstraints(ifStmt *ast.IfStmt, registry *protoregistry.Types, want featurev1beta1.FeatureType) []*featurev1beta1.Constraint {
	constraint := &featurev1beta1.Constraint{}
	constraint.RuleAstNew = exprToRule(ifStmt.Cond)
	assert.Equal(len(ifStmt.Body.List), 1, "if statements can only contain one return statement")
	returnStmt, ok := ifStmt.Body.List[0].(*ast.ReturnStmt) // TODO
	assert.Equal(ok, true, "if statements can only contain return statements")
	constraint.Value = exprToAny(returnStmt.Results[0], registry, want) // TODO
	if ifStmt.Else != nil {                                             // TODO bare else?
		elseIfStmt, ok := ifStmt.Else.(*ast.IfStmt)
		assert.Equal(ok, true, "bare else statements are not supported, must be else if")
		return append([]*featurev1beta1.Constraint{constraint}, ifToConstraints(elseIfStmt, registry, want)...)
	}
	return []*featurev1beta1.Constraint{constraint}
}

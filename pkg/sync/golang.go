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

	"path"
	"strconv"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/assert"
	"github.com/lainio/err2/try"
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
		if d.Name() == "lekko.go" { // TODO: Change file name to be based off namespace
			syncer := NewGoSyncer(mf.Module.Mod.Path, p, repoPath)
			if err := syncer.Sync(ctx); err != nil {
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

type goSyncer struct {
	moduleRoot string
	lekkoPath  string
	filePath   string // Path to Go source file to sync
	repoPath   string // Path to config repository on local fs

	typeRegistry  *protoregistry.Types
	protoPackages map[string]string // Map of local package names to protobuf packages (e.g. configv1beta1 -> default.config.v1beta1)
}

func NewGoSyncer(moduleRoot, filePath, repoPath string) *goSyncer {
	return &goSyncer{
		moduleRoot: moduleRoot,
		// Assumes target file is at <lekkoPath>/<namespace>/<file>
		lekkoPath:     filepath.Clean(filepath.Dir(filepath.Dir(filePath))),
		filePath:      filepath.Clean(filePath),
		repoPath:      repoPath,
		protoPackages: make(map[string]string),
	}
}

func (g *goSyncer) Sync(ctx context.Context) error {
	src, err := os.ReadFile(g.filePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("open %s", g.filePath))
	}
	if bytes.Contains(src, []byte("<<<<<<<")) {
		return fmt.Errorf("%s has unresolved merge conflicts", g.filePath)
	}

	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, g.filePath, src, parser.ParseComments)
	if err != nil {
		return err
	}
	namespace := Namespace{}

	r, err := repo.NewLocal(g.repoPath, nil)
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
	g.typeRegistry = registry

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
			// Analyze imports to create mapping of proto packages
			// Assumes proto packages are under <lekkoPath>/proto
			// and that proto package follows folder structure (e.g. default/config/v1beta1 <-> default.config.v1beta1)
			protoDir := filepath.Join(g.moduleRoot, g.lekkoPath, "proto")
			for _, is := range x.Imports {
				if strings.Contains(is.Path.Value, protoDir) {
					if is.Name == nil {
						panic("protobuf imports must explicitly specify package aliases")
					}
					relProtoDir := try.To1(filepath.Rel(protoDir, strings.Trim(is.Path.Value, "\"'")))
					protoPackage := strings.ReplaceAll(relProtoDir, "/", ".")
					g.protoPackages[is.Name.Name] = protoPackage
				}
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
							panic("unexpected default value already processed")
						}
						// TODO also need to take care of the possibility that the default is in an else
						feature.Tree.Default = g.exprToAny(n.Results[0], feature.Type) // can this be multiple things?
					case *ast.IfStmt:
						feature.Tree.Constraints = append(feature.Tree.Constraints, g.ifToConstraints(n, feature.Type)...)
					default:
						panic("only if and return statements allowed in function body")
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
func (g *goSyncer) exprToAny(expr ast.Expr, want featurev1beta1.FeatureType) *anypb.Any {
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
				a, err := anypb.New(g.compositeLitToProto(x).(protoreflect.ProtoMessage))
				if err != nil {
					panic(errors.Wrap(err, "marshal Any"))
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

// e.g. configv1beta1.Message -> [configv1beta1, Message]
func exprToNameParts(expr ast.Expr) []string {
	switch node := expr.(type) {
	case *ast.Ident:
		return []string{node.Name}
	case *ast.SelectorExpr:
		return append(exprToNameParts(node.X), exprToNameParts(node.Sel)...)
	default:
		panic(fmt.Errorf("invalid expression for name %+v", node))
	}
}

func (g *goSyncer) compositeLitToMessageType(x *ast.CompositeLit) protoreflect.MessageType {
	innerIdent, ok := x.Type.(*ast.SelectorExpr).X.(*ast.Ident)
	if ok && innerIdent.Name == "durationpb" {
		mt, err := g.typeRegistry.FindMessageByName(protoreflect.FullName("google.protobuf").Append(protoreflect.Name(x.Type.(*ast.SelectorExpr).Sel.Name)))
		if err == nil {
			return mt
		}
		panic(err)
	}

	parts := exprToNameParts(x.Type)
	assert.Equal(len(parts), 2, fmt.Sprintf("expected message name to be 2 parts: %v", parts))
	protoPackage, ok := g.protoPackages[parts[0]]
	assert.Equal(ok, true, fmt.Sprintf("unknown package %v", parts[0]))
	fullName := protoreflect.FullName(protoPackage).Append(protoreflect.Name(parts[1]))
	mt, err := g.typeRegistry.FindMessageByName(fullName)
	if errors.Is(err, protoregistry.NotFound) {
		// Check if nested type (e.g. Outer_Inner) (only works 2 levels for now)
		if strings.Contains(parts[1], "_") {
			names := strings.Split(parts[1], "_")
			assert.Equal(len(names), 2, fmt.Sprintf("only singly nested messages are supported: %v", parts[1]))
			if outerDescriptor, err := g.typeRegistry.FindMessageByName(protoreflect.FullName(protoPackage).Append(protoreflect.Name(names[0]))); err == nil {
				if innerDescriptor := outerDescriptor.Descriptor().Messages().ByName(protoreflect.Name(names[1])); innerDescriptor != nil {
					return dynamicpb.NewMessageType(innerDescriptor)
				}
			}
		}
		panic(fmt.Errorf("missing proto type in registry %s", fullName))
	} else if err != nil {
		panic(errors.Wrap(err, "error while finding message type registry"))
	} else {
		return mt
	}
}

func primitiveToProtoValue(expr ast.Expr) any {
	switch x := expr.(type) {
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING:
			return strings.Trim(x.Value, "\"`")
		case token.INT:
			// TODO - parse/validate based on field Kind, because this breaks for
			// int32, etc. fields
			if intValue, err := strconv.ParseInt(x.Value, 10, 64); err == nil {
				return intValue
			} else {
				panic(errors.Wrapf(err, "64-bit int parse token %s", x.Value))
			}
		case token.FLOAT:
			if floatValue, err := strconv.ParseFloat(x.Value, 64); err == nil {
				return floatValue
			} else {
				panic(errors.Wrapf(err, "float parse token %s", x.Value))
			}
		default:
			// Booleans are handled separately as literal identifiers below
			panic(fmt.Errorf("unsupported basic literal token type %v", x.Kind))
		}
	case *ast.Ident:
		switch x.Name {
		case "true":
			return true
		case "false":
			return false
		default:
			panic(fmt.Errorf("unsupported identifier %v", x.Name))
		}
	default:
		panic(fmt.Errorf("expected primitive expression, got %+v", x))
	}
}

func (g *goSyncer) compositeLitToProto(x *ast.CompositeLit) protoreflect.Message {
	mt := g.compositeLitToMessageType(x)
	msg := mt.New()
	for _, v := range x.Elts {
		kv, ok := v.(*ast.KeyValueExpr)
		assert.Equal(ok, true)
		keyIdent, ok := kv.Key.(*ast.Ident)
		assert.Equal(ok, true)
		name := strcase.ToSnake(keyIdent.Name)
		field := mt.Descriptor().Fields().ByName(protoreflect.Name(name))
		if field == nil {
			panic(fmt.Errorf("missing field descriptor for %v", name))
		}
		switch node := kv.Value.(type) {
		case *ast.UnaryExpr:
			switch node.Op {
			case token.AND:
				switch ix := node.X.(type) {
				case *ast.CompositeLit:
					msg.Set(field, protoreflect.ValueOf(g.compositeLitToProto(ix)))
				default:
					panic(fmt.Errorf("unsupported X type for unary & %T", ix))
				}
			default:
				panic(fmt.Errorf("unsupported unary operator %v", node.Op))
			}
		case *ast.CompositeLit:
			switch clTypeNode := node.Type.(type) {
			case *ast.ArrayType:
				lVal := msg.Mutable(field).List()
				switch eltTypeNode := clTypeNode.Elt.(type) {
				case *ast.Ident:
					// Primitive type array
					for _, elt := range node.Elts {
						eltVal := primitiveToProtoValue(elt)
						lVal.Append(protoreflect.ValueOf(eltVal))
					}
				case *ast.StarExpr:
					// Proto type array
					// For type, need to process e.g. *configv1beta1.SomeMessage
					selectorExpr, ok := eltTypeNode.X.(*ast.SelectorExpr)
					assert.Equal(ok, true, "expected slice type like *package.Message, got %+v", eltTypeNode.X)
					for _, e := range node.Elts {
						var cl *ast.CompositeLit
						switch elt := e.(type) {
						case *ast.CompositeLit:
							// Directly a composite literal means no type
							// HACK: take overall slice's type expression and set it on the composite literal
							// because if element is directly a composite literal, the Type field is nil
							// e.g. []*pkg.Type{&pkg.Type{Field: ...}, &pkg.Type{Field: ...}} vs. []*pkg.Type{{Field: ...}, {Field: ...}}
							elt.Type = selectorExpr
							cl = elt
						case *ast.UnaryExpr:
							switch elt.Op {
							case token.AND:
								switch ux := elt.X.(type) {
								case *ast.CompositeLit:
									cl = ux
								default:
									panic(fmt.Errorf("unsupported X type for unary & %T", ux))
								}
							default:
								panic(fmt.Errorf("unsupported unary operator %v", elt.Op))
							}
						default:
							panic(fmt.Errorf("unsupported slice element type %+v", elt))
						}
						lVal.Append(protoreflect.ValueOf(g.compositeLitToProto(cl)))
					}
				default:
					panic(fmt.Errorf("unsupported slice element type %+v", eltTypeNode))
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
					assert.Equal(ok, true, "expected key value expression for map element")
					basicLit, ok := pair.Key.(*ast.BasicLit)
					assert.Equal(ok, true, "expected basic literal for map key")
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
			field.Kind()
			// Value is not a composite literal - try handling as a primitive
			value := primitiveToProtoValue(node)
			if field.Kind() == protoreflect.EnumKind {
				// Special handling for enums
				intValue, ok := value.(int64)
				assert.Equal(ok, true, "expected int value")
				msg.Set(field, protoreflect.ValueOf(protoreflect.EnumNumber(intValue)))
				continue
			}
			msg.Set(field, protoreflect.ValueOf(value))
		}
	}
	return msg
}

// TODO: Use new primitive helper instead
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
		_, ok := node.Type.(*ast.ArrayType)
		assert.Equal(ok, true, "only slices are allowed for composite literals in comparisons")
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

func (g *goSyncer) ifToConstraints(ifStmt *ast.IfStmt, want featurev1beta1.FeatureType) []*featurev1beta1.Constraint {
	constraint := &featurev1beta1.Constraint{}
	constraint.RuleAstNew = exprToRule(ifStmt.Cond)
	assert.Equal(len(ifStmt.Body.List), 1, "if statements can only contain one return statement")
	returnStmt, ok := ifStmt.Body.List[0].(*ast.ReturnStmt) // TODO
	assert.Equal(ok, true, "if statements can only contain return statements")
	constraint.Value = g.exprToAny(returnStmt.Results[0], want) // TODO
	if ifStmt.Else != nil {                                     // TODO bare else?
		elseIfStmt, ok := ifStmt.Else.(*ast.IfStmt)
		assert.Equal(ok, true, "bare else statements are not supported, must be else if")
		return append([]*featurev1beta1.Constraint{constraint}, g.ifToConstraints(elseIfStmt, want)...)
	}
	return []*featurev1beta1.Constraint{constraint}
}

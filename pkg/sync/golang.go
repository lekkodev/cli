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
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

	"strconv"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/assert"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func BisyncGo(ctx context.Context, outputPath, lekkoPath, repoPath string) ([]string, error) {
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
		if err != nil {
			return err
		}
		// Skip generated proto dir
		if d.IsDir() && d.Name() == "proto" {
			return filepath.SkipDir
		}
		// Sync and gen - only target <namespace>/<namespace>.go files
		if !d.IsDir() && strings.TrimSuffix(d.Name(), ".go") == filepath.Base(filepath.Dir(p)) {
			files = append(files, p)
			fmt.Printf("Successfully bisynced %s\n", logging.Bold(p))
		}
		// Ignore others
		return nil
	}); err != nil {
		return nil, err
	}
	syncer := NewGoSyncer()
	repoContents, err := syncer.Sync(files...)
	if err != nil {
		return nil, errors.Wrap(err, "sync")
	}
	if err := WriteContentsToLocalRepo(ctx, repoContents, repoPath); err != nil {
		return nil, err
	}
	generator, err := gen.NewGoGenerator(mf.Module.Mod.Path, outputPath, lekkoPath, repoContents)
	if err != nil {
		return nil, errors.Wrap(err, "initialize code generator")
	}
	for _, namespace := range repoContents.Namespaces {
		if err := generator.Gen(ctx, namespace.Name); err != nil {
			return nil, errors.Wrapf(err, "generate code for %s", namespace.Name)
		}
	}
	return files, nil
}

func GetDependencies(descriptor *descriptorpb.DescriptorProto) []string {
	dependencies := make(map[string]struct{})

	for _, field := range descriptor.GetField() {
		// Assume that field types that are fully qualified (e.g. .google.protobuf.Duration) need to be imported
		// as opposed to locally available (e.g. LocalType)
		// This isn't true in all cases, because a type in the same package will still need to be imported
		// if it was defined in a separate file
		// But since we control local file generation, we probably don't have to worry about it
		// TODO: handle enum field type
		if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			if strings.HasPrefix(field.GetTypeName(), ".") {
				dependencies[field.GetTypeName()] = struct{}{}
			}
		}
	}

	for _, nested := range descriptor.GetNestedType() {
		nestedDeps := GetDependencies(nested)
		for _, dep := range nestedDeps {
			dependencies[dep] = struct{}{}
		}
	}

	// Convert map to slice
	var depList []string
	for dep := range dependencies {
		// TODO: this only works for very specific cases where expected filename == message name, e.g. .google.protobuf.Duration -> google/protobuf/duration.proto
		// We should change to returning fullnames instead of paths then try to look up paths when registering imports in file descriptor downstream
		depList = append(depList, strings.ToLower(strings.Replace(dep[1:], ".", "/", -1)+".proto"))
	}

	return depList
}

// Registers a message descriptor for a namespace in the passed FDS
func registerMessage(fds *descriptorpb.FileDescriptorSet, mdp *descriptorpb.DescriptorProto, namespace string) error {
	filePath := fmt.Sprintf("%s/config/v1beta1/%s.proto", namespace, namespace)
	// Try to find existing file descriptor
	var fdp *descriptorpb.FileDescriptorProto
	for _, file := range fds.File {
		if file.GetName() == filePath {
			fdp = file
		}
	}
	if fdp == nil {
		// Create new if necessary
		fdp = &descriptorpb.FileDescriptorProto{
			Name:    proto.String(filePath),
			Package: proto.String(fmt.Sprintf("%s.config.v1beta1", namespace)),
		}
		fds.File = append(fds.File, fdp)
	}
	// Add message descriptor proto (and check for duplicate register)
	for _, message := range fdp.MessageType {
		if mdp.GetName() == message.GetName() {
			return fmt.Errorf("duplicate registration of message %s.%s", fdp.GetPackage(), message.GetName())
		}
	}
	fdp.MessageType = append(fdp.MessageType, mdp)
	// Add messages' dependencies to file's dependencies
	mDeps := GetDependencies(mdp)
	// Prevent duplicates
	for _, mDep := range mDeps {
		found := false
		for _, fDep := range fdp.Dependency {
			if fDep == mDep {
				found = true
				break
			}
		}
		if !found {
			fdp.Dependency = append(fdp.Dependency, mDep)
		}
	}

	return nil
}

// Translates Go code to representation of a Lekko repository's contents
type goSyncer struct {
	fset *token.FileSet
}

func NewGoSyncer() *goSyncer {
	return &goSyncer{
		fset: token.NewFileSet(),
	}
}

// As a side effect, mutates the passed FileDescriptorSet to register types parsed from the AST.
func (g *goSyncer) AstToNamespace(pf *ast.File, fds *descriptorpb.FileDescriptorSet) (*featurev1beta1.Namespace, error) {
	// TODO: instead of panicking everywhere, collect errors (maybe using go/analysis somehow)
	// so we can report them properly (and not look sketchy)
	namespace := &featurev1beta1.Namespace{}
	// First pass to get general metadata and register all types
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
			return true
		case *ast.GenDecl:
			// TODO: try to handle doc comments using x.Doc and protoreflect.SourceLocation
			for _, spec := range x.Specs {
				if _, ok := spec.(*ast.ImportSpec); ok {
					return false
				}
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					panic(g.posErr(x, "only type declarations are supported"))
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					panic(g.posErr(typeSpec, "only struct type declarations are supported"))
				}
				d := g.structToDescriptor(typeSpec.Name.Name, structType)
				err := registerMessage(fds, d, namespace.Name)
				if err != nil {
					panic(g.posErr(typeSpec, "failed to register type for struct"))
				}
			}
			return true
		default:
			return false
		}
	})
	// At this point, we should have processed all types
	tr, err := protoutils.FileDescriptorSetToTypeRegistry(fds)
	if err != nil {
		return nil, errors.Wrap(err, "pre-process type registry")
	}
	// Second pass to handle all functions
	ast.Inspect(pf, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			// TODO: We should support numbers (e.g. v2) but the strcase pkg has some non-ideal behavior with numbers,
			// we might want to write our own librar(ies) with cross-language consistency
			if regexp.MustCompile("^[gG]et[A-Z][A-Za-z]*$").MatchString(x.Name.Name) {
				var commentLines []string
				if x.Doc != nil {
					for _, comment := range x.Doc.List {
						commentLines = append(commentLines, strings.TrimLeft(comment.Text, "/ "))
					}
				}
				privateName := x.Name.Name // TODO - not sure how we use this, but does it work right with letting people just declare GetFoo and letting them be happy?
				configName := strcase.ToKebab(privateName[3:])
				feature := &featurev1beta1.Feature{Key: configName, Description: strings.Join(commentLines, " "), Tree: &featurev1beta1.Tree{}}
				namespace.Features = append(namespace.Features, feature)
				contextKeys := make(map[string]string)

				structName, structType := FindArgStruct(x, pf)
				if structType != nil {
					feature.SignatureTypeUrl = fmt.Sprintf("type.googleapis.com/%s.config.v1beta1.%s", namespace.Name, structName)
					contextKeys = StructToMap(structType)
				} else {
					for _, param := range x.Type.Params.List {
						assert.SNotEmpty(param.Names, "must have a parameter name")
						assert.INotNil(param.Type, "must have a parameter type")
						typeIdent, ok := param.Type.(*ast.Ident)
						if !ok {
							panic(g.posErr(param, errors.New("parameter type must be an identifier")))
						}
						contextKeys[param.Names[0].Name] = typeIdent.Name
					}
				}

				results := x.Type.Results.List
				if results == nil {
					panic(g.posErr(x, "must have a return type"))
				}
				if len(results) != 1 {
					panic(g.posErr(x, "must have exactly one return type"))
				}

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
						// TODO - check if it is one of our structs to allow non *
						panic(g.posErr(t, fmt.Sprintf("unsupported primitive return type %s", t.Name)))
					}
				case *ast.StarExpr:
					feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_PROTO
				default:
					panic(g.posErr(t, fmt.Errorf("unsupported return type expression %+v", t)))
				}
				for _, stmt := range x.Body.List {
					switch n := stmt.(type) {
					case *ast.ReturnStmt:
						if feature.Tree.Default != nil {
							panic(g.posErr(n, "unexpected default value already processed"))
						}
						// TODO also need to take care of the possibility that the default is in an else
						feature.Tree.DefaultNew = g.exprToAny(n.Results[0], feature.Type, namespace.Name, tr) // can this be multiple things?
					case *ast.IfStmt:
						feature.Tree.Constraints = append(feature.Tree.Constraints, g.ifToConstraints(n, feature.Type, contextKeys, namespace.Name, tr)...)
					default:
						panic(g.posErr(n, "only if and return statements allowed in function body"))
					}
				}
				return false
			}
			panic(g.posErr(x.Name, "only function names like 'getConfig' are supported"))
		}
		return true
	})
	// TODO static context
	return namespace, nil
}

// Translate a collection of Go files to a representation of repository contents.
// Files -> repo instead of file -> namespace because FDS is shared repo-wide.
// Takes file paths instead of contents for more helpful error reporting.
func (g *goSyncer) Sync(filePaths ...string) (*featurev1beta1.RepositoryContents, error) {
	ret := &featurev1beta1.RepositoryContents{FileDescriptorSet: protoutils.NewDefaultFileDescriptorSet()}
	for _, filePath := range filePaths {
		astf, err := parser.ParseFile(g.fset, filePath, nil, parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution)
		if err != nil {
			return nil, errors.Wrapf(err, "parse %s", filePath)
		}
		ns, err := g.AstToNamespace(astf, ret.FileDescriptorSet)
		if err != nil {
			return nil, errors.Wrapf(err, "translate %s", filePath)
		}
		ret.Namespaces = append(ret.Namespaces, ns)
	}

	return ret, nil
}

// TODO - is this only used for context keys, or other things?
func (g *goSyncer) exprToValue(expr ast.Expr) string {
	//fmt.Printf("%+v\n", expr)
	switch v := expr.(type) {
	case *ast.Ident:
		return strcase.ToSnake(v.Name)
	case *ast.SelectorExpr:
		return strcase.ToSnake(v.Sel.Name)
	default:
		panic("Invalid syntax")
	}
}

// TODO -- We know the return type..
func (g *goSyncer) exprToAny(expr ast.Expr, want featurev1beta1.FeatureType, namespace string, typeRegistry *protoregistry.Types) *featurev1beta1.Any {
	switch node := expr.(type) {
	case *ast.UnaryExpr:
		switch node.Op {
		case token.AND:
			switch x := node.X.(type) {
			case *ast.CompositeLit:
				// TODO - this is the one place we set the values for return types
				message, overrides := g.compositeLitToProto(x, namespace, typeRegistry)
				protoMsg, ok := message.(protoreflect.ProtoMessage)
				if !ok {
					panic("This should never happen")
				}
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(protoMsg)
				if err != nil {
					panic(err)
				}
				return &featurev1beta1.Any{
					TypeUrl:   "type.googleapis.com/" + string(protoMsg.ProtoReflect().Descriptor().FullName()),
					Value:     value,
					Overrides: overrides,
				}
			default:
				panic(fmt.Errorf("unsupported unary & target %+v", x))
			}
		default:
			panic(fmt.Errorf("unsupported unary operator %v", node.Op))
		}
	case *ast.CallExpr:
		if fun, ok := node.Fun.(*ast.Ident); ok {
			// TODO ensure that the arguments are the same as the main calling function
			// TODO -- do the other stuff...
			configKey := strcase.ToKebab(fun.Name[3:])
			protoMsg := &featurev1beta1.ConfigCall{
				Namespace: namespace,
				Key:       configKey,
				// TODO do we know the location of this?
			}
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(protoMsg)
			if err != nil {
				panic(err)
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/" + string(protoMsg.ProtoReflect().Descriptor().FullName()),
				Value:   value,
			}
		} else {
			panic(fmt.Errorf("unsupported function call expression %+v", node))
		}
	default:
		// TODO
		value := g.primitiveToProtoValue(expr, namespace)
		switch typedValue := value.(type) {
		case string:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.StringValue{Value: typedValue})
			if err != nil {
				panic(err)
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.StringValue",
				Value:   value,
			}
		case int64:
			// A value parsed as an integer might actually be for a float config
			switch want {
			case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.Int64Value{Value: typedValue})
				if err != nil {
					panic(err)
				}
				return &featurev1beta1.Any{
					TypeUrl: "type.googleapis.com/google.protobuf.Int64Value",
					Value:   value,
				}
			case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
				// TODO: handle precision boundaries properly
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: float64(typedValue)})
				if err != nil {
					panic(err)
				}
				return &featurev1beta1.Any{
					TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
					Value:   value,
				}
			default:
				panic(fmt.Errorf("unexpected primitive %v for target return type %v", typedValue, want))
			}
		case float64:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: typedValue})
			if err != nil {
				panic(err)
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
				Value:   value,
			}
		case bool:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.BoolValue{Value: typedValue})
			if err != nil {
				panic(err)
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.BoolValue",
				Value:   value,
			}
		default:
			panic(fmt.Errorf("unsupported value expression %+v", node))
		}
	}
}

// TODO: Handling for duration and nested types in general are really complex, buggy and not well tested.
// We should probably start with spec'ing out the type constructs we're willing to support in all native languages.
func (g *goSyncer) compositeLitToMessageType(x *ast.CompositeLit, namespace string, typeRegistry *protoregistry.Types) protoreflect.MessageType {
	var fullName protoreflect.FullName
	innerExpr, ok := x.Type.(*ast.SelectorExpr)
	if ok {
		innerIdent, ok := innerExpr.X.(*ast.Ident)
		if ok && innerIdent.Name == "durationpb" {
			mt, err := typeRegistry.FindMessageByName(protoreflect.FullName("google.protobuf").Append(protoreflect.Name(innerExpr.Sel.Name)))
			if err == nil {
				return mt
			}
			panic(err)
		}
		panic(errors.New("unsupported selector expression for composite literal type"))
	} else {
		// it should be an ident for a bare raw struct
		ident, ok := x.Type.(*ast.Ident)
		if !ok {
			panic("Unknown syntax")
		}
		// TODO - fix this - this is gross af
		fullName = protoreflect.FullName(fmt.Sprintf("%s.config.v1beta1", namespace)).Append(protoreflect.Name(ident.Name))
		mt, err := typeRegistry.FindMessageByName(fullName)
		if err != nil {
			panic(errors.Wrapf(err, "find %s in type registry", fullName))
		} else {
			return mt
		}
	}
}

func (g *goSyncer) primitiveToProtoValue(expr ast.Expr, namespace string) any {
	switch x := expr.(type) {
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING:
			// Need to unescape escaped - Unquote also handles escaped chars in middle
			// and is fine with alternate quotes like ' or `
			if unescaped, err := strconv.Unquote(x.Value); err == nil {
				return unescaped
			} else {
				panic(errors.Wrapf(err, "unescape string literal %s", x.Value))
			}
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
	case *ast.CallExpr:
		if fun, ok := x.Fun.(*ast.Ident); ok {
			// TODO ensure that the arguments are the same as the main calling function
			// TODO -- do the other stuff...
			configKey := strcase.ToKebab(fun.Name[3:])
			return &featurev1beta1.ConfigCall{
				Namespace: namespace,
				Key:       configKey,
				// TODO do we know the location of this?
			}
		} else {
			panic(fmt.Errorf("unsupported function call expression %+v", x))
		}
	default:
		panic(fmt.Errorf("expected primitive expression, got %+v", x))
	}
}

func (g *goSyncer) compositeLitToProto(x *ast.CompositeLit, namespace string, typeRegistry *protoregistry.Types) (protoreflect.Message, []*featurev1beta1.ValueOveride) {
	var overrides []*featurev1beta1.ValueOveride
	mt := g.compositeLitToMessageType(x, namespace, typeRegistry)
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
					innerMessage, calls := g.compositeLitToProto(ix, namespace, typeRegistry)
					if len(calls) > 0 {
						fmt.Printf("%+v\n", calls)
					}
					msg.Set(field, protoreflect.ValueOf(innerMessage))
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
						eltVal := g.primitiveToProtoValue(elt, namespace)
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
						innerMessage, innerOverrides := g.compositeLitToProto(cl, namespace, typeRegistry)
						overrides = append(overrides, innerOverrides...)
						lVal.Append(protoreflect.ValueOf(innerMessage))
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
					// For now, assume all map values are primitives
					value := g.primitiveToProtoValue(pair.Value, namespace)
					msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(value))
				}
			default:
				panic(fmt.Errorf("unsupported composite literal type %T", clTypeNode))
			}
		default:
			// Value is not a composite literal - try handling as a primitive
			value := g.primitiveToProtoValue(node, namespace)
			if field.Kind() == protoreflect.EnumKind {
				// Special handling for enums
				intValue, ok := value.(int64)
				assert.Equal(ok, true, "expected int value")
				msg.Set(field, protoreflect.ValueOf(protoreflect.EnumNumber(intValue)))
				continue
			}
			// convert int64 to float64 (double) if this is what proto expects
			// it's not 100% safe, but should be fine for values < 9007199254740993
			if intValue, ok := value.(int64); ok && field.Kind() == protoreflect.DoubleKind {
				value = float64(intValue)
			}
			if configCall, ok := value.(*featurev1beta1.ConfigCall); ok {
				override := &featurev1beta1.ValueOveride{
					Call:      configCall,
					FieldPath: []int32{int32(field.Number())}, // TODO
				}
				overrides = append(overrides, override)
			} else if value != nil {
				msg.Set(field, protoreflect.ValueOf(value))
			}
		}
	}
	return msg, overrides
}

func (g *goSyncer) exprToComparisonValue(expr ast.Expr, namespace string) *structpb.Value {
	switch node := expr.(type) {
	case *ast.CompositeLit:
		_, ok := node.Type.(*ast.ArrayType)
		assert.Equal(ok, true, "only slices are allowed for composite literals in comparisons")
		var list []*structpb.Value
		for _, elt := range node.Elts {
			list = append(list, g.exprToComparisonValue(elt, namespace))
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: list,
				},
			},
		}
	default:
		// If not composite lit, must(/should) be primitive
		value := g.primitiveToProtoValue(expr, namespace)
		ret := &structpb.Value{}
		switch typedValue := value.(type) {
		case string:
			ret.Kind = &structpb.Value_StringValue{
				StringValue: typedValue,
			}
		case int64:
			ret.Kind = &structpb.Value_NumberValue{
				NumberValue: float64(typedValue),
			}
		case float64:
			ret.Kind = &structpb.Value_NumberValue{
				NumberValue: typedValue,
			}
		case bool:
			ret.Kind = &structpb.Value_BoolValue{
				BoolValue: typedValue,
			}
		default:
			panic(fmt.Errorf("unexpected type for primitive value %v", typedValue))
		}
		return ret
	}
}

func (g *goSyncer) binaryExprToRule(expr *ast.BinaryExpr, contextKeys map[string]string, namespace string) *rulesv1beta3.Rule {
	switch expr.Op {
	case token.LAND:
		var rules []*rulesv1beta3.Rule
		left := g.exprToRule(expr.X, contextKeys, namespace)
		l, ok := left.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && l.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND {
			rules = append(rules, l.LogicalExpression.Rules...)
		} else {
			rules = append(rules, left)
		}
		right := g.exprToRule(expr.Y, contextKeys, namespace)
		r, ok := right.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && r.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND {
			rules = append(rules, r.LogicalExpression.Rules...)
		} else {
			rules = append(rules, right)
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND, Rules: rules}}}
	case token.LOR:
		var rules []*rulesv1beta3.Rule
		left := g.exprToRule(expr.X, contextKeys, namespace)
		l, ok := left.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && l.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR {
			rules = append(rules, l.LogicalExpression.Rules...)
		} else {
			rules = append(rules, left)
		}
		right := g.exprToRule(expr.Y, contextKeys, namespace)
		r, ok := right.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && r.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR {
			rules = append(rules, r.LogicalExpression.Rules...)
		} else {
			rules = append(rules, right)
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR, Rules: rules}}}
	case token.EQL:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	case token.LSS:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	case token.GTR:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	case token.NEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	case token.LEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	case token.GEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS, ContextKey: g.exprToValue(expr.X), ComparisonValue: g.exprToComparisonValue(expr.Y, namespace)}}}
	default:
		panic(fmt.Errorf("unexpected token in binary expression %v", expr.Op))
	}
}

func (g *goSyncer) callExprToRule(expr *ast.CallExpr, namespace string) *rulesv1beta3.Rule {
	// TODO check Fun
	selectorExpr, ok := expr.Fun.(*ast.SelectorExpr)
	assert.Equal(ok, true)
	ident, ok := selectorExpr.X.(*ast.Ident)
	assert.Equal(ok, true)
	switch ident.Name { // TODO: is there a way to differentiate between an expr on a package vs. a struct/interface? could give better error messages
	case "slices":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ContextKey: g.exprToValue(expr.Args[1]), ComparisonValue: g.exprToComparisonValue(expr.Args[0], namespace)}}}
		default:
			panic(fmt.Errorf("unsupported slices operator %s", selectorExpr.Sel.Name))
		}
	case "strings":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS, ContextKey: g.exprToValue(expr.Args[0]), ComparisonValue: g.exprToComparisonValue(expr.Args[1], namespace)}}}
		case "HasPrefix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH, ContextKey: g.exprToValue(expr.Args[0]), ComparisonValue: g.exprToComparisonValue(expr.Args[1], namespace)}}}
		case "HasSuffix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH, ContextKey: g.exprToValue(expr.Args[0]), ComparisonValue: g.exprToComparisonValue(expr.Args[1], namespace)}}}
		default:
			panic(fmt.Errorf("unsupported strings operator %s", selectorExpr.Sel.Name))
		}
	default:
		panic(fmt.Errorf("unexpected identifier in rule %s", ident.Name))
	}
}

func (g *goSyncer) unaryExprToRule(expr *ast.UnaryExpr, contextKeys map[string]string, namespace string) *rulesv1beta3.Rule {
	switch expr.Op {
	case token.NOT:
		rule := g.exprToRule(expr.X, contextKeys, namespace)
		if atom := rule.GetAtom(); atom != nil {
			boolValue, isBool := atom.ComparisonValue.GetKind().(*structpb.Value_BoolValue)
			if isBool && atom.ComparisonOperator == rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS {
				atom.ComparisonValue = structpb.NewBoolValue(!boolValue.BoolValue)
			}
			return rule
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Not{Not: rule}}
	default:
		panic(fmt.Errorf("unsupported unary expression %+v", expr))
	}
}

func (g *goSyncer) identToRule(ident *ast.Ident, contextKeys map[string]string) *rulesv1beta3.Rule {
	if contextKeyType, ok := contextKeys[ident.Name]; ok && contextKeyType == "bool" {
		return &rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: &rulesv1beta3.Atom{
					ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
					ContextKey:         strcase.ToSnake(ident.Name),
					ComparisonValue:    structpb.NewBoolValue(true),
				},
			},
		}
	}
	panic(fmt.Errorf("not a boolean expression: %+v", ident))
}

func (g *goSyncer) exprToRule(expr ast.Expr, contextKeys map[string]string, namespace string) *rulesv1beta3.Rule {
	switch node := expr.(type) {
	case *ast.Ident:
		return g.identToRule(node, contextKeys)
	case *ast.BinaryExpr:
		return g.binaryExprToRule(node, contextKeys, namespace)
	case *ast.CallExpr:
		return g.callExprToRule(node, namespace)
	case *ast.ParenExpr:
		return g.exprToRule(node.X, contextKeys, namespace)
	case *ast.UnaryExpr:
		return g.unaryExprToRule(node, contextKeys, namespace)
	case *ast.SelectorExpr: // TODO - make sure this is args
		return g.identToRule(node.Sel, contextKeys)
	default:
		panic(fmt.Errorf("unsupported expression type for rule: %T", node))
	}
}

func (g *goSyncer) ifToConstraints(ifStmt *ast.IfStmt, want featurev1beta1.FeatureType, contextKeys map[string]string, namespace string, typeRegistry *protoregistry.Types) []*featurev1beta1.Constraint {
	constraint := &featurev1beta1.Constraint{}
	constraint.RuleAstNew = g.exprToRule(ifStmt.Cond, contextKeys, namespace)
	assert.Equal(len(ifStmt.Body.List), 1, "if statements can only contain one return statement")
	returnStmt, ok := ifStmt.Body.List[0].(*ast.ReturnStmt) // TODO
	assert.Equal(ok, true, "if statements can only contain return statements")
	constraint.ValueNew = g.exprToAny(returnStmt.Results[0], want, namespace, typeRegistry) // TODO
	if ifStmt.Else != nil {                                                                 // TODO bare else?
		elseIfStmt, ok := ifStmt.Else.(*ast.IfStmt)
		assert.Equal(ok, true, "bare else statements are not supported, must be else if")
		return append([]*featurev1beta1.Constraint{constraint}, g.ifToConstraints(elseIfStmt, want, contextKeys, namespace, typeRegistry)...)
	}
	return []*featurev1beta1.Constraint{constraint}
}

func (g *goSyncer) structToDescriptor(structName string, structType *ast.StructType) *descriptorpb.DescriptorProto {
	descriptor := &descriptorpb.DescriptorProto{}
	descriptor.Name = proto.String(structName)
	for i, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			panic(fmt.Sprintf("struct %s: field must only have one name", structName))
		}
		fieldName := field.Names[0].Name
		fieldDescriptor := &descriptorpb.FieldDescriptorProto{
			Name:   proto.String(strcase.ToSnake(fieldName)),
			Number: proto.Int32(int32(i + 1)),
			Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
		}
		switch fieldType := field.Type.(type) {
		case *ast.Ident:
			switch fieldType.Name {
			case "int64":
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
			case "string":
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			case "float64":
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
			case "bool":
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
			default:
				panic(fmt.Sprintf("unsupported field type %s for %s.%s", fieldType.Name, structName, fieldName))
			}
		case *ast.SelectorExpr:
			// Special handling for durationpb.Duration
			if pkgIdent, ok := fieldType.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && fieldType.Sel.Name == "Duration" {
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
				fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
			} else {
				panic(fmt.Sprintf("unsupported selector field type for %s.%s", structName, fieldName))
			}
		case *ast.StarExpr:
			// Handle durationpb.Duration type
			if selectorExpr, ok := fieldType.X.(*ast.SelectorExpr); ok {
				if pkgIdent, ok := selectorExpr.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && selectorExpr.Sel.Name == "Duration" {
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
					fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
				} else {
					panic(fmt.Sprintf("unsupported star expression type for %s.%s", structName, fieldName))
				}
			} else {
				panic(fmt.Sprintf("sunsupported star expression type for %s.%s", structName, fieldName))
			}
		case *ast.MapType:
			keyIdent, ok := fieldType.Key.(*ast.Ident)
			if !ok {
				panic("fieldType.Key is not of type *ast.Ident")
			}
			keyType := keyIdent.Name
			valueIdent, ok := fieldType.Value.(*ast.Ident)
			if !ok {
				panic("fieldType.Value is not of type *ast.Ident")
			}
			valueType := valueIdent.Name
			mapEntryDescriptor := &descriptorpb.DescriptorProto{
				Name: proto.String(fieldName + "Entry"),
			}

			keyField := &descriptorpb.FieldDescriptorProto{
				Name:   proto.String("key"),
				Number: proto.Int32(1),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			}
			valueField := &descriptorpb.FieldDescriptorProto{
				Name:   proto.String("value"),
				Number: proto.Int32(2),
				Label:  descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			}

			switch keyType {
			case "int64":
				keyField.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
			case "string":
				keyField.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			case "float64":
				keyField.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
			case "bool":
				keyField.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
			default:
				panic("unknown map key type in struct")
			}

			switch valueType {
			case "int64":
				valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
			case "string":
				valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
			case "float64":
				valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
			case "bool":
				valueField.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
			default:
				panic("unknown map value type in struct")
			}

			mapEntryDescriptor.Field = append(mapEntryDescriptor.Field, keyField, valueField)
			mapEntryDescriptor.Options = &descriptorpb.MessageOptions{
				MapEntry: proto.Bool(true),
			}

			descriptor.NestedType = append(descriptor.NestedType, mapEntryDescriptor)
			fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
			fieldDescriptor.TypeName = proto.String(fieldName + "Entry")
			fieldDescriptor.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
		case *ast.ArrayType:
			fieldDescriptor.Label = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			elemType := fieldType.Elt
			switch elemType := elemType.(type) {
			case *ast.Ident:
				switch elemType.Name {
				case "int64":
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
				case "string":
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum()
				case "float64":
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
				case "bool":
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
				default:
					panic(fmt.Sprintf("unsupported array element type %s for %s.%s", elemType.Name, structName, fieldName))
				}
			case *ast.SelectorExpr:
				if pkgIdent, ok := elemType.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && elemType.Sel.Name == "Duration" {
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
					fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
				} else {
					panic(fmt.Sprintf("unsupported selector type array element for %s.%s", structName, fieldName))
				}
			default:
				panic(fmt.Sprintf("unsupported array element type for %s.%s", structName, fieldName))
			}
		default:
			panic(fmt.Sprintf("unsupported field type for %s.%s", structName, fieldName))
		}
		descriptor.Field = append(descriptor.Field, fieldDescriptor)
	}
	return descriptor
}

// Wrap an error related to an AST node with positional information
func (g *goSyncer) posErr(node ast.Node, err any) error {
	var inner error
	switch e := err.(type) {
	case string:
		inner = errors.New(e)
	case error:
		inner = e
	default:
		panic("invalid inner error type")
	}
	p := g.fset.Position(node.Pos())
	return errors.Wrapf(inner, "error at %s:%d:%d", p.Filename, p.Line, p.Column)
}

func StructToMap(structType *ast.StructType) map[string]string {
	ret := make(map[string]string)
	for _, field := range structType.Fields.List {
		for _, fieldName := range field.Names {
			switch fieldType := field.Type.(type) {
			case *ast.Ident:
				ret[fieldName.Name] = fieldType.Name
			default:
				panic("not a struct I understand")
			}
		}
	}
	return ret
}

func FindArgStruct(f *ast.FuncDecl, file *ast.File) (string, *ast.StructType) {
	if f.Type.Params.NumFields() != 1 {
		return "", nil
	}

	param := f.Type.Params.List[0]
	starExpr, ok := param.Type.(*ast.StarExpr)
	if !ok {
		return "", nil
	}

	ident, ok := starExpr.X.(*ast.Ident)
	if !ok {
		return "", nil
	}

	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			structType, ok := typeSpec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			if typeSpec.Name.Name == ident.Name {
				return ident.Name, structType
			}
		}
	}
	return "", nil
}

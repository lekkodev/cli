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
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/gen"
	"github.com/lekkodev/cli/pkg/logging"
	"github.com/pkg/errors"
	"golang.org/x/mod/modfile"

	"strconv"

	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2"
	"github.com/lainio/err2/try"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func BisyncGo(ctx context.Context, outputPath, lekkoPath, repoOwner, repoName, repoPath string) ([]string, error) {
	b, err := os.ReadFile("go.mod")
	if err != nil {
		return nil, errors.Wrap(err, "find go.mod in working directory")
	}
	mf, err := modfile.ParseLax("go.mod", b, nil)
	if err != nil {
		return nil, err
	}
	// Traverse target path, finding namespaces
	files := make([]string, 0)
	if err := filepath.WalkDir(lekkoPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip generated proto dir
		if d.IsDir() && d.Name() == "proto" {
			return filepath.SkipDir
		}
		// Only target <namespace>/<namespace>.go files
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
		return nil, errors.Wrap(err, "write to repository")
	}
	generator, err := gen.NewGoGenerator(mf.Module.Mod.Path, lekkoPath, repoOwner, repoName, repoContents)
	if err != nil {
		return nil, errors.Wrap(err, "initialize code generator")
	}
	generated, err := generator.Gen(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "gen")
	}
	if err := generated.WriteFiles(outputPath); err != nil {
		return nil, errors.Wrap(err, "write generated files")
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
func (g *goSyncer) AstToNamespace(pf *ast.File, fds *descriptorpb.FileDescriptorSet) (namespace *featurev1beta1.Namespace, retErr error) {
	defer err2.Handle(&retErr, nil)
	// TODO: instead of panicking everywhere, collect errors (maybe using go/analysis somehow)
	// so we can report them properly (and not look sketchy)
	namespace = &featurev1beta1.Namespace{}
	// First pass to get general metadata and register all types
	ast.Inspect(pf, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.File:
			// i.e. lekkodefault -> default (this requires the package name to be correct)
			if !strings.HasPrefix(x.Name.Name, "lekko") {
				retErr = g.posErr(x, "packages for lekko must start with 'lekko'")
				return false
			}
			namespace.Name = x.Name.Name[5:]
			if len(namespace.Name) == 0 {
				retErr = g.posErr(x, "namespace name cannot be empty")
				return false
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
					retErr = g.posErr(x, "only type declarations are supported")
					return false
				}
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					retErr = g.posErr(typeSpec, "only struct type declarations are supported")
					return false
				}
				d := try.To1(g.structToDescriptor(typeSpec.Name.Name, structType))
				err := registerMessage(fds, d, namespace.Name)
				if err != nil {
					retErr = g.posErr(typeSpec, "failed to register type for struct")
					return false
				}
			}
			return true
		default:
			return false
		}
	})
	if retErr != nil {
		return nil, retErr
	}
	// At this point, we should have processed all types
	tr, retErr := protoutils.FileDescriptorSetToTypeRegistry(fds)
	if retErr != nil {
		return nil, errors.Wrap(retErr, "pre-process type registry")
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
				feature := &featurev1beta1.Feature{Key: configName, Description: strings.Join(commentLines, "\n"), Tree: &featurev1beta1.Tree{}}
				namespace.Features = append(namespace.Features, feature)
				contextKeys := make(map[string]string)

				structName, structType := FindArgStruct(x, pf)
				if structType != nil {
					feature.SignatureTypeUrl = fmt.Sprintf("type.googleapis.com/%s.config.v1beta1.%s", namespace.Name, structName)
					contextKeys = try.To1(g.structToMap(structType))
				} else {
					for _, param := range x.Type.Params.List {
						if len(param.Names) < 1 {
							retErr = g.posErr(param, "parameter names must be present")
							return false
						}
						if param.Type == nil {
							retErr = g.posErr(param, "parameter type must be present")
							return false
						}
						typeIdent, ok := param.Type.(*ast.Ident)
						if !ok {
							retErr = g.posErr(param, errors.New("parameter type must be an identifier"))
							return false
						}
						contextKeys[param.Names[0].Name] = typeIdent.Name
					}
				}

				results := x.Type.Results.List
				if results == nil {
					retErr = g.posErr(x, "must have a return type")
					return false
				}
				if len(results) != 1 {
					retErr = g.posErr(x, "must have exactly one return type")
					return false
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
						retErr = g.posErr(t, fmt.Sprintf("unsupported primitive return type %s", t.Name))
						return false
					}
				case *ast.StarExpr:
					feature.Type = featurev1beta1.FeatureType_FEATURE_TYPE_PROTO
				default:
					retErr = g.posErr(t, fmt.Errorf("unsupported return type expression %+v", t))
					return false
				}
				for _, stmt := range x.Body.List {
					switch n := stmt.(type) {
					case *ast.ReturnStmt:
						if feature.Tree.Default != nil {
							retErr = g.posErr(n, "unexpected default value already processed")
							return false
						}
						// TODO also need to take care of the possibility that the default is in an else
						// TODO: Maybe a bug -  exprToAny should check that type matches intended
						feature.Tree.DefaultNew = try.To1(g.exprToAny(n.Results[0], feature.Type, namespace.Name, tr)) // can this be multiple things?
					case *ast.IfStmt:
						feature.Tree.Constraints = append(feature.Tree.Constraints, try.To1(g.ifToConstraints(n, feature.Type, contextKeys, namespace.Name, tr))...)
					default:
						retErr = g.posErr(n, "only if and return statements allowed in function body")
						return false
					}
				}
				return false
			}
			retErr = g.posErr(x.Name, "only function names like 'getConfig' are supported")
			return false
		}
		return true
	})

	// Final sort features inside namespace for consistent ordering
	slices.SortFunc(namespace.Features, func(a, b *featurev1beta1.Feature) int {
		if a.Key < b.Key {
			return -1
		}
		if a.Key > b.Key {
			return 1
		}
		return 0
	})

	// TODO static context
	return namespace, nil
}

func sortRepositoryContents(contents *featurev1beta1.RepositoryContents) {
	slices.SortFunc(contents.Namespaces, func(a, b *featurev1beta1.Namespace) int {
		if a.Name < b.Name {
			return -1
		}
		if a.Name > b.Name {
			return 1
		}
		return 0
	})
	for _, ns := range contents.Namespaces {
		slices.SortFunc(ns.Features, func(a, b *featurev1beta1.Feature) int {
			if a.Key < b.Key {
				return -1
			}
			if a.Key > b.Key {
				return 1
			}
			return 0
		})
	}
}

// Translate a collection of Go files to a representation of repository contents.
// Files -> repo instead of file -> namespace because FDS is shared repo-wide.
func (g *goSyncer) Sync(filePaths ...string) (*featurev1beta1.RepositoryContents, error) {
	ret := &featurev1beta1.RepositoryContents{FileDescriptorSet: protoutils.NewDefaultFileDescriptorSet()}
	for _, filePath := range filePaths {
		astf, err := parser.ParseFile(g.fset, filePath, nil, parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution)
		if err != nil {
			return nil, errors.Wrapf(g.parseParseError(err), "parse %s", filePath)
		}
		ns, err := g.AstToNamespace(astf, ret.FileDescriptorSet)
		if err != nil {
			return nil, errors.Wrapf(err, "translate %s", filePath)
		}
		ret.Namespaces = append(ret.Namespaces, ns)
	}
	sortRepositoryContents(ret)

	return ret, nil
}

// Translates Go code contents to a representation of repository contents.
// Expects a map of namespace names to corresponding private file code.
// Technically, the namespace is included in the package name which is in the code (unlike TypeScript)
// but keeping this signature for now for simplicity.
func (g *goSyncer) SyncContents(contentMap map[string]string) (*featurev1beta1.RepositoryContents, error) {
	ret := &featurev1beta1.RepositoryContents{FileDescriptorSet: protoutils.NewDefaultFileDescriptorSet()}
	for namespace, contents := range contentMap {
		astf, err := parser.ParseFile(g.fset, namespace, contents, parser.ParseComments|parser.AllErrors|parser.SkipObjectResolution)
		if err != nil {
			return nil, errors.Wrapf(g.parseParseError(err), "parse %s", namespace)
		}
		ns, err := g.AstToNamespace(astf, ret.FileDescriptorSet)
		if err != nil {
			return nil, errors.Wrapf(err, "translate %s", namespace)
		}
		ret.Namespaces = append(ret.Namespaces, ns)
	}
	sortRepositoryContents(ret)

	return ret, nil
}

func (g *goSyncer) parseParseError(err error) error {
	if perr, ok := err.(scanner.ErrorList); ok && len(perr) >= 1 {
		return NewSyncPosError(errors.New(perr[0].Msg), perr[0].Pos.Filename, perr[0].Pos.Line, perr[0].Pos.Column, perr[0].Pos.Line, perr[0].Pos.Column+1)
	} else if err == nil {
		return nil
	}
	return NewSyncError(err)
}

// TODO - is this only used for context keys, or other things?
func (g *goSyncer) exprToValue(expr ast.Expr) (string, error) {
	switch v := expr.(type) {
	case *ast.Ident:
		return strcase.ToSnake(v.Name), nil
	case *ast.SelectorExpr:
		return strcase.ToSnake(v.Sel.Name), nil
	default:
		return "", g.posErr(expr, "unsupported context key syntax")
	}
}

// TODO -- We know the return type..
func (g *goSyncer) exprToAny(expr ast.Expr, want featurev1beta1.FeatureType, namespace string, typeRegistry *protoregistry.Types) (a *featurev1beta1.Any, err error) {
	switch node := expr.(type) {
	case *ast.UnaryExpr:
		switch node.Op {
		case token.AND:
			switch x := node.X.(type) {
			case *ast.CompositeLit:
				// TODO - this is the one place we set the values for return types
				message, overrides := try.To2(g.compositeLitToProto(x, namespace, typeRegistry))
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(message.Interface())
				if err != nil {
					return nil, errors.Wrap(err, "marshal composite lit proto")
				}
				return &featurev1beta1.Any{
					TypeUrl:   "type.googleapis.com/" + string(message.Interface().ProtoReflect().Descriptor().FullName()),
					Value:     value,
					Overrides: overrides,
				}, nil
			default:
				return nil, g.posErr(x, "unsupported unary & target")
			}
		case token.SUB:
			// Handle negative numbers
			switch want {
			case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
				val, err := g.primitiveToProtoValue(node.X, namespace)
				if err != nil {
					return nil, err
				}
				if intVal, ok := val.(int64); !ok {
					return nil, g.posErr(node, "unsupported unary minus operator for non-integer literal")
				} else {
					value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.Int64Value{Value: -1 * intVal})
					if err != nil {
						return nil, errors.Wrap(err, "marshal Int64Value")
					}
					return &featurev1beta1.Any{
						TypeUrl: "type.googleapis.com/google.protobuf.Int64Value",
						Value:   value,
					}, nil
				}
			case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
				val, err := g.primitiveToProtoValue(node.X, namespace)
				if err != nil {
					return nil, err
				}
				switch typedValue := val.(type) {
				case int64:
					value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: -1 * float64(typedValue)})
					if err != nil {
						return nil, errors.Wrap(err, "marshal DoubleValue")
					}
					return &featurev1beta1.Any{
						TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
						Value:   value,
					}, nil
				case float64:
					value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: -1 * float64(typedValue)})
					if err != nil {
						return nil, errors.Wrap(err, "marshal DoubleValue")
					}
					return &featurev1beta1.Any{
						TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
						Value:   value,
					}, nil
				default:
					return nil, g.posErr(node, "unsupported unary minus operator for non-numeric literal")
				}
			default:
				return nil, g.posErr(node, "unsupported minus operator for non-numeric type")
			}
		default:
			return nil, g.posErr(node, "unsupported unary operator")
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
				return nil, errors.Wrap(err, "marshal ConfigCall")
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/" + string(protoMsg.ProtoReflect().Descriptor().FullName()),
				Value:   value,
			}, nil
		} else {
			return nil, g.posErr(node.Fun, "unsupported function call expression")
		}
	default:
		// TODO
		value := try.To1(g.primitiveToProtoValue(expr, namespace))
		switch typedValue := value.(type) {
		case string:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.StringValue{Value: typedValue})
			if err != nil {
				return nil, errors.Wrap(err, "marshal StringValue")
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.StringValue",
				Value:   value,
			}, nil
		case int64:
			// A value parsed as an integer might actually be for a float config
			switch want {
			case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.Int64Value{Value: typedValue})
				if err != nil {
					return nil, errors.Wrap(err, "marshal Int64Value")
				}
				return &featurev1beta1.Any{
					TypeUrl: "type.googleapis.com/google.protobuf.Int64Value",
					Value:   value,
				}, nil
			case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
				// TODO: handle precision boundaries properly
				value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: float64(typedValue)})
				if err != nil {
					return nil, errors.Wrap(err, "marshal DoubleValue")
				}
				return &featurev1beta1.Any{
					TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
					Value:   value,
				}, nil
			default:
				return nil, g.posErr(node, fmt.Errorf("unexpected primitive for return type %v", want))
			}
		case float64:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.DoubleValue{Value: typedValue})
			if err != nil {
				return nil, errors.Wrap(err, "marshal DoubleValue")
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.DoubleValue",
				Value:   value,
			}, nil
		case bool:
			value, err := proto.MarshalOptions{Deterministic: true}.Marshal(&wrapperspb.BoolValue{Value: typedValue})
			if err != nil {
				return nil, errors.Wrap(err, "marshal BoolValue")
			}
			return &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.BoolValue",
				Value:   value,
			}, nil
		default:
			return nil, g.posErr(node, "unsupported value expression")
		}
	}
}

// TODO: Handling for duration and nested types in general are really complex, buggy and not well tested.
// We should probably start with spec'ing out the type constructs we're willing to support in all native languages.
func (g *goSyncer) compositeLitToMessageType(x *ast.CompositeLit, namespace string, typeRegistry *protoregistry.Types) (protoreflect.MessageType, error) {
	var fullName protoreflect.FullName
	innerExpr, ok := x.Type.(*ast.SelectorExpr)
	if ok {
		innerIdent, ok := innerExpr.X.(*ast.Ident)
		if ok && innerIdent.Name == "durationpb" {
			fullName := protoreflect.FullName("google.protobuf").Append(protoreflect.Name(innerExpr.Sel.Name))
			mt, err := typeRegistry.FindMessageByName(fullName)
			if err != nil {
				return nil, errors.Wrapf(err, "find message %s", fullName)
			}
			return mt, nil
		}
		return nil, g.posErr(innerExpr, "unsupported selector expression for composite literal type")
	} else {
		// it should be an ident for a bare raw struct
		ident, ok := x.Type.(*ast.Ident)
		if !ok {
			return nil, g.posErr(x, "unsupported syntax")
		}
		// TODO - fix this - this is gross af
		fullName = protoreflect.FullName(fmt.Sprintf("%s.config.v1beta1", namespace)).Append(protoreflect.Name(ident.Name))
		mt, err := typeRegistry.FindMessageByName(fullName)
		if err != nil {
			return nil, errors.Wrapf(err, "find %s in type registry", fullName)
		}
		return mt, nil
	}
}

func (g *goSyncer) primitiveToProtoValue(expr ast.Expr, namespace string) (any, error) {
	switch x := expr.(type) {
	case *ast.BasicLit:
		switch x.Kind {
		case token.STRING:
			// Need to unescape escaped - Unquote also handles escaped chars in middle
			// and is fine with alternate quotes like ' or `
			if unescaped, err := strconv.Unquote(x.Value); err == nil {
				return unescaped, nil
			} else {
				return nil, g.posErr(x, "invalid string literal")
			}
		case token.INT:
			// TODO - parse/validate based on field Kind, because this breaks for
			// int32, etc. fields
			if intValue, err := strconv.ParseInt(x.Value, 10, 64); err == nil {
				return intValue, nil
			} else {
				return nil, g.posErr(x, "invalid int literal")
			}
		case token.FLOAT:
			if floatValue, err := strconv.ParseFloat(x.Value, 64); err == nil {
				return floatValue, nil
			} else {
				return nil, g.posErr(x, "invalid float literal")
			}
		default:
			// Booleans are handled separately as literal identifiers below
			return nil, g.posErr(x, "unsupported basic literal type")
		}
	case *ast.Ident:
		switch x.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return nil, g.posErr(x, "unsupported identifier")
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
			}, nil
		} else {
			return nil, g.posErr(x, "unsupported function call expression")
		}
	default:
		return nil, g.posErr(x, "expected primitive expression")
	}
}

func (g *goSyncer) compositeLitToProto(x *ast.CompositeLit, namespace string, typeRegistry *protoregistry.Types) (msg protoreflect.Message, vos []*featurev1beta1.ValueOveride, err error) {
	var overrides []*featurev1beta1.ValueOveride
	mt := try.To1(g.compositeLitToMessageType(x, namespace, typeRegistry))
	msg = mt.New()
	for _, v := range x.Elts {
		kv, ok := v.(*ast.KeyValueExpr)
		if !ok {
			return nil, nil, g.posErr(v, "expected key value expression")
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			return nil, nil, g.posErr(kv.Key, "expected identifier")
		}
		name := strcase.ToSnake(keyIdent.Name)
		field := mt.Descriptor().Fields().ByName(protoreflect.Name(name))
		if field == nil {
			return nil, nil, g.posErr(keyIdent, fmt.Errorf("unexpected field, missing field descriptor %s", name))
		}
		switch node := kv.Value.(type) {
		case *ast.UnaryExpr:
			switch node.Op {
			case token.AND:
				switch ix := node.X.(type) {
				case *ast.CompositeLit:
					innerMessage, calls := try.To2(g.compositeLitToProto(ix, namespace, typeRegistry))
					if len(calls) > 0 {
						fmt.Printf("%+v\n", calls)
					}
					msg.Set(field, protoreflect.ValueOf(innerMessage))
				default:
					return nil, nil, g.posErr(ix, "unsupported target type for unary & expression")
				}
			default:
				return nil, nil, g.posErr(node, "unsupported unary operator")
			}
		case *ast.CompositeLit:
			switch clTypeNode := node.Type.(type) {
			case *ast.ArrayType:
				lVal := msg.Mutable(field).List()
				switch eltTypeNode := clTypeNode.Elt.(type) {
				case *ast.Ident:
					// Primitive type array
					for _, elt := range node.Elts {
						eltVal := try.To1(g.primitiveToProtoValue(elt, namespace))
						lVal.Append(protoreflect.ValueOf(eltVal))
					}
				case *ast.StarExpr:
					// Proto type array
					// For type, need to process e.g. *configv1beta1.SomeMessage
					selectorExpr, ok := eltTypeNode.X.(*ast.SelectorExpr)
					if !ok {
						return nil, nil, g.posErr(eltTypeNode.X, "expected slice type like *package.Message")
					}
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
									return nil, nil, g.posErr(ux, "unsupported target type for unary & expression")
								}
							default:
								return nil, nil, g.posErr(elt, "unsupported unary operator")
							}
						default:
							return nil, nil, g.posErr(elt, "unsupported slice element type")
						}
						innerMessage, innerOverrides := try.To2(g.compositeLitToProto(cl, namespace, typeRegistry))
						overrides = append(overrides, innerOverrides...)
						lVal.Append(protoreflect.ValueOf(innerMessage))
					}
				default:
					return nil, nil, g.posErr(eltTypeNode, "unsupported slice element type")
				}
			case *ast.MapType:
				mapTypeNode := clTypeNode
				// TODO: Currently only supports primitive kvs
				switch mapTypeNode.Key.(type) {
				case *ast.Ident:
					// Do something
				default:
					return nil, nil, g.posErr(mapTypeNode.Key, "unsupported map key type")
				}
				switch mapTypeNode.Value.(type) {
				case *ast.Ident:
					// Do something
				default:
					return nil, nil, g.posErr(mapTypeNode.Value, "unsupported map value type")
				}
				for _, elt := range node.Elts {
					pair, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						return nil, nil, g.posErr(elt, "expected key value expression for map element")
					}
					basicLit, ok := pair.Key.(*ast.BasicLit)
					if !ok {
						return nil, nil, g.posErr(pair.Key, "expected basic literal for map key")
					}
					key := protoreflect.ValueOfString(strings.Trim(basicLit.Value, "\"")).MapKey()
					// For now, assume all map values are primitives
					value := try.To1(g.primitiveToProtoValue(pair.Value, namespace))
					msg.Mutable(field).Map().Set(key, protoreflect.ValueOf(value))
				}
			default:
				return nil, nil, g.posErr(clTypeNode, fmt.Errorf("unsupported composite literal type %T", clTypeNode))
			}
		default:
			// Value is not a composite literal - try handling as a primitive
			value := try.To1(g.primitiveToProtoValue(node, namespace))
			if field.Kind() == protoreflect.EnumKind {
				// Special handling for enums
				intValue, ok := value.(int64)
				if !ok {
					return nil, nil, g.posErr(node, "expected int value")
				}
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
	return msg, overrides, nil
}

func (g *goSyncer) exprToComparisonValue(expr ast.Expr, namespace string) (v *structpb.Value, err error) {
	switch node := expr.(type) {
	case *ast.CompositeLit:
		_, ok := node.Type.(*ast.ArrayType)
		if !ok {
			return nil, g.posErr(node.Type, "only slices are allowed for composite literals in comparisons")
		}
		var list []*structpb.Value
		for _, elt := range node.Elts {
			list = append(list, try.To1(g.exprToComparisonValue(elt, namespace)))
		}
		return &structpb.Value{
			Kind: &structpb.Value_ListValue{
				ListValue: &structpb.ListValue{
					Values: list,
				},
			},
		}, nil
	default:
		// If not composite lit, must(/should) be primitive
		value := try.To1(g.primitiveToProtoValue(expr, namespace))
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
			return nil, g.posErr(node, "unexpected type for primitive value")
		}
		return ret, nil
	}
}

func (g *goSyncer) binaryExprToRule(expr *ast.BinaryExpr, contextKeys map[string]string, namespace string) (rule *rulesv1beta3.Rule, err error) {
	switch expr.Op {
	case token.LAND:
		var rules []*rulesv1beta3.Rule
		left := try.To1(g.exprToRule(expr.X, contextKeys, namespace))
		l, ok := left.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && l.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND {
			rules = append(rules, l.LogicalExpression.Rules...)
		} else {
			rules = append(rules, left)
		}
		right := try.To1(g.exprToRule(expr.Y, contextKeys, namespace))
		r, ok := right.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && r.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND {
			rules = append(rules, r.LogicalExpression.Rules...)
		} else {
			rules = append(rules, right)
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_AND, Rules: rules}}}, nil
	case token.LOR:
		var rules []*rulesv1beta3.Rule
		left := try.To1(g.exprToRule(expr.X, contextKeys, namespace))
		l, ok := left.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && l.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR {
			rules = append(rules, l.LogicalExpression.Rules...)
		} else {
			rules = append(rules, left)
		}
		right := try.To1(g.exprToRule(expr.Y, contextKeys, namespace))
		r, ok := right.Rule.(*rulesv1beta3.Rule_LogicalExpression)
		if ok && r.LogicalExpression.LogicalOperator == rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR {
			rules = append(rules, r.LogicalExpression.Rules...)
		} else {
			rules = append(rules, right)
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_LogicalExpression{LogicalExpression: &rulesv1beta3.LogicalExpression{LogicalOperator: rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR, Rules: rules}}}, nil
	case token.EQL:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	case token.LSS:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	case token.GTR:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	case token.NEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	case token.LEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	case token.GEQ:
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS, ContextKey: try.To1(g.exprToValue(expr.X)), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Y, namespace))}}}, nil
	default:
		return nil, g.posErr(expr, "unsupported binary operator")
	}
}

func (g *goSyncer) callExprToRule(expr *ast.CallExpr, namespace string) (rule *rulesv1beta3.Rule, err error) {
	// TODO check Fun
	selectorExpr, ok := expr.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, g.posErr(expr.Fun, "unsupported function expression type, expected selector expression")
	}
	ident, ok := selectorExpr.X.(*ast.Ident)
	if !ok {
		return nil, g.posErr(selectorExpr.X, "unsupported expression type, expected identifier")
	}
	switch ident.Name { // TODO: is there a way to differentiate between an expr on a package vs. a struct/interface? could give better error messages
	case "slices":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ContextKey: try.To1(g.exprToValue(expr.Args[1])), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Args[0], namespace))}}}, nil
		default:
			return nil, g.posErr(selectorExpr.Sel, "unsupported slices operator")
		}
	case "strings":
		switch selectorExpr.Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS, ContextKey: try.To1(g.exprToValue(expr.Args[0])), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Args[1], namespace))}}}, nil
		case "HasPrefix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH, ContextKey: try.To1(g.exprToValue(expr.Args[0])), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Args[1], namespace))}}}, nil
		case "HasSuffix":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH, ContextKey: try.To1(g.exprToValue(expr.Args[0])), ComparisonValue: try.To1(g.exprToComparisonValue(expr.Args[1], namespace))}}}, nil
		default:
			return nil, g.posErr(selectorExpr, fmt.Errorf("unsupported strings operator %s", selectorExpr.Sel.Name))
		}
	default:
		return nil, g.posErr(ident, fmt.Errorf("unexpected identifier in rule %s", ident.Name))
	}
}

func (g *goSyncer) unaryExprToRule(expr *ast.UnaryExpr, contextKeys map[string]string, namespace string) (*rulesv1beta3.Rule, error) {
	switch expr.Op {
	case token.NOT:
		rule, err := g.exprToRule(expr.X, contextKeys, namespace)
		if err != nil {
			return nil, err
		}
		if atom := rule.GetAtom(); atom != nil {
			boolValue, isBool := atom.ComparisonValue.GetKind().(*structpb.Value_BoolValue)
			if isBool && atom.ComparisonOperator == rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS {
				atom.ComparisonValue = structpb.NewBoolValue(!boolValue.BoolValue)
			}
			return rule, nil
		}
		return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Not{Not: rule}}, nil
	default:
		return nil, g.posErr(expr, "unsupported unary expression type")
	}
}

func (g *goSyncer) identToRule(ident *ast.Ident, contextKeys map[string]string) (*rulesv1beta3.Rule, error) {
	if contextKeyType, ok := contextKeys[ident.Name]; ok && contextKeyType == "bool" {
		return &rulesv1beta3.Rule{
			Rule: &rulesv1beta3.Rule_Atom{
				Atom: &rulesv1beta3.Atom{
					ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS,
					ContextKey:         strcase.ToSnake(ident.Name),
					ComparisonValue:    structpb.NewBoolValue(true),
				},
			},
		}, nil
	}
	return nil, g.posErr(ident, "not a boolean expression")
}

func (g *goSyncer) exprToRule(expr ast.Expr, contextKeys map[string]string, namespace string) (*rulesv1beta3.Rule, error) {
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
		return nil, g.posErr(node, "unsupported expressioin type for rule")
	}
}

func (g *goSyncer) ifToConstraints(ifStmt *ast.IfStmt, want featurev1beta1.FeatureType, contextKeys map[string]string, namespace string, typeRegistry *protoregistry.Types) (constraints []*featurev1beta1.Constraint, err error) {
	constraint := &featurev1beta1.Constraint{}
	constraint.RuleAstNew = try.To1(g.exprToRule(ifStmt.Cond, contextKeys, namespace))
	if len(ifStmt.Body.List) != 1 {
		return nil, g.posErr(ifStmt.Body, "if statements can only contain one return statement")
	}
	returnStmt, ok := ifStmt.Body.List[0].(*ast.ReturnStmt) // TODO
	if !ok {
		return nil, g.posErr(ifStmt.Body.List[0], "if statements can only contain return statements")
	}
	constraint.ValueNew = try.To1(g.exprToAny(returnStmt.Results[0], want, namespace, typeRegistry)) // TODO
	if ifStmt.Else != nil {                                                                          // TODO bare else?
		elseIfStmt, ok := ifStmt.Else.(*ast.IfStmt)
		if !ok {
			return nil, g.posErr(ifStmt.Else, "bare else statements are not supported, must be else if")
		}
		return append([]*featurev1beta1.Constraint{constraint}, try.To1(g.ifToConstraints(elseIfStmt, want, contextKeys, namespace, typeRegistry))...), nil
	}
	return []*featurev1beta1.Constraint{constraint}, nil
}

func (g *goSyncer) structToDescriptor(structName string, structType *ast.StructType) (*descriptorpb.DescriptorProto, error) {
	descriptor := &descriptorpb.DescriptorProto{}
	descriptor.Name = proto.String(structName)
	for i, field := range structType.Fields.List {
		if len(field.Names) != 1 {
			return nil, g.posErr(field, "field must only have one name")
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
				return nil, g.posErr(field.Type, "unsupported field type")
			}
		case *ast.SelectorExpr:
			// Special handling for durationpb.Duration
			if pkgIdent, ok := fieldType.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && fieldType.Sel.Name == "Duration" {
				fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
				fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
			} else {
				return nil, g.posErr(fieldType, "unsupported selector field type")
			}
		case *ast.StarExpr:
			// Handle durationpb.Duration type
			if selectorExpr, ok := fieldType.X.(*ast.SelectorExpr); ok {
				if pkgIdent, ok := selectorExpr.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && selectorExpr.Sel.Name == "Duration" {
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
					fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
				} else {
					return nil, g.posErr(selectorExpr, "unsupported star expression type")
				}
			} else {
				return nil, g.posErr(fieldType, "unsupported star expression type")
			}
		case *ast.MapType:
			keyIdent, ok := fieldType.Key.(*ast.Ident)
			if !ok {
				return nil, g.posErr(fieldType.Key, "expected identifier for map key")
			}
			keyType := keyIdent.Name
			valueIdent, ok := fieldType.Value.(*ast.Ident)
			if !ok {
				return nil, g.posErr(fieldType.Value, "expected identifier for map value")
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
				return nil, g.posErr(keyIdent, "unsupported map key type in struct")
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
				return nil, g.posErr(valueIdent, "unsupported map value type in struct")
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
					return nil, g.posErr(elemType, "unsupported array element type")
				}
			case *ast.SelectorExpr:
				if pkgIdent, ok := elemType.X.(*ast.Ident); ok && pkgIdent.Name == "durationpb" && elemType.Sel.Name == "Duration" {
					fieldDescriptor.Type = descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum()
					fieldDescriptor.TypeName = proto.String(".google.protobuf.Duration")
				} else {
					return nil, g.posErr(elemType.X, "unsupported selector type array element")
				}
			default:
				return nil, g.posErr(elemType, "unsupported array element type")
			}
		default:
			return nil, g.posErr(fieldType, "unsupported field type")
		}
		descriptor.Field = append(descriptor.Field, fieldDescriptor)
	}
	return descriptor, nil
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
		// This means there's an internal bug
		panic("invalid inner error type")
	}
	startPos := g.fset.Position(node.Pos())
	endPos := g.fset.Position(node.End())
	return NewSyncPosError(inner, startPos.Filename, startPos.Line, startPos.Column, endPos.Line, endPos.Column)
}

func (g *goSyncer) structToMap(structType *ast.StructType) (map[string]string, error) {
	ret := make(map[string]string)
	for _, field := range structType.Fields.List {
		for _, fieldName := range field.Names {
			switch fieldType := field.Type.(type) {
			case *ast.Ident:
				ret[fieldName.Name] = fieldType.Name
			default:
				return nil, g.posErr(field.Type, "unsupported field type")
			}
		}
	}
	return ret, nil
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

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

package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
    "path"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
    "github.com/lekkodev/cli/pkg/star"
      "github.com/lekkodev/go-sdk/pkg/eval"
  "github.com/pkg/errors"
    "github.com/lekkodev/cli/pkg/feature"
     "github.com/lekkodev/cli/pkg/star/static"
	"github.com/iancoleman/strcase"
	"github.com/lekkodev/cli/pkg/repo"
	"github.com/lekkodev/cli/pkg/secrets"
	"github.com/spf13/cobra"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "sync code to config",
	}
	cmd.AddCommand(syncGoCmd())
	return cmd
}

// TODO - make this our proto rep?
type Namespace struct {
	Name     string
	Features []*featurev1beta1.Feature
}

func exprToValue(expr ast.Expr) string {
	// This works better than it should...
	return fmt.Sprintf("%s", expr)
}

// TODO -- We know the return type..
func exprToAny(expr ast.Expr, registry *protoregistry.Types, want featurev1beta1.FeatureType) *anypb.Any {
	switch node := expr.(type) {
  case *ast.BasicLit:
    switch node.Kind {
    case token.STRING:
      a, err := anypb.New(&wrapperspb.StringValue{Value: strings.Trim(node.Value, "\"")})
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

func compositeLitToProto(x *ast.CompositeLit, registry *protoregistry.Types) protoreflect.Message {
	mt, err := registry.FindMessageByName(protoreflect.FullName("default.config.v1beta1").Append(protoreflect.Name(x.Type.(*ast.SelectorExpr).Sel.Name)))
	if err != nil {
		// TODO - google wkt crap
		fmt.Printf("%s: %s\n", err, x.Type)
		panic(err)
	} else {
		msg := mt.New()
		for _, v := range x.Elts {
			kv := v.(*ast.KeyValueExpr)
			name := strcase.ToSnake(kv.Key.(*ast.Ident).Name)
			field := mt.Descriptor().Fields().ByName(protoreflect.Name(name))
			switch node := kv.Value.(type) {
			case *ast.BasicLit:
				switch node.Kind {
				case token.STRING:
					msg.Set(field, protoreflect.ValueOf(strings.Trim(node.Value, "\"")))
				case token.INT:
					if intValue, err := strconv.ParseInt(node.Value, 10, 64); err == nil {
						if err != nil {
							panic(err)
						}
						msg.Set(field, protoreflect.ValueOf(intValue))
					}
				case token.FLOAT:
					if floatValue, err := strconv.ParseFloat(node.Value, 64); err == nil {
						if err != nil {
							panic(err)
						}
						msg.Set(field, protoreflect.ValueOf(floatValue))
					}
				default:
					fmt.Printf("NV: %s\n", node.Value)
				}
			case *ast.Ident:
				switch node.Name {
				case "true":
					msg.Set(field, protoreflect.ValueOf(true))
				case "false":
					msg.Set(field, protoreflect.ValueOf(false))
				default:
					fmt.Printf("NN: %s\n", node.Name)
				}
			case *ast.UnaryExpr:
				switch node.Op {
				case token.AND:
					switch ix := node.X.(type) {
					case *ast.CompositeLit:
						msg.Set(field, protoreflect.ValueOf(compositeLitToProto(ix, registry)))
					default:
						panic("Unknown X Type")
					}
				default:
					panic("Unknown Op")
				}
			default:
				fmt.Printf("ETP: %#v\n", node)
			}

		}
		return msg
	}
}

func exprToComparisonValue(expr ast.Expr) *structpb.Value {
	switch node := expr.(type) {
	case *ast.BasicLit:
		switch node.Kind {
		case token.STRING:
			return &structpb.Value{
				Kind: &structpb.Value_StringValue{
					StringValue: strings.Trim(node.Value, "\""),
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
	switch expr.Fun.(*ast.SelectorExpr).X.(*ast.Ident).Name { // TODO... brittle..
	case "slices":
		switch expr.Fun.(*ast.SelectorExpr).Sel.Name {
		case "Contains":
			return &rulesv1beta3.Rule{Rule: &rulesv1beta3.Rule_Atom{Atom: &rulesv1beta3.Atom{ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN, ContextKey: exprToValue(expr.Args[1]), ComparisonValue: exprToComparisonValue(expr.Args[0])}}}
		default:
			panic("Ahhhh")
		}
	case "strings":
		switch expr.Fun.(*ast.SelectorExpr).Sel.Name {
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
	return &rulesv1beta3.Rule{}
}

func exprToRule(expr ast.Expr) *rulesv1beta3.Rule {
	switch node := expr.(type) {
	case *ast.BinaryExpr:
		return binaryExprToRule(node)
	case *ast.CallExpr:
		return callExprToRule(node)
	default:
		panic("Panic!!")
	}
}

func ifToConstraints(ifStmt *ast.IfStmt, registry *protoregistry.Types, want featurev1beta1.FeatureType) []*featurev1beta1.Constraint {
	constraint := &featurev1beta1.Constraint{}
	constraint.RuleAstNew = exprToRule(ifStmt.Cond)
	constraint.Value = exprToAny(ifStmt.Body.List[0].(*ast.ReturnStmt).Results[0], registry, want) // TODO
	if ifStmt.Else != nil {                                                                  // TODO bare else?
		return append([]*featurev1beta1.Constraint{constraint}, ifToConstraints(ifStmt.Else.(*ast.IfStmt), registry, want)...)
	}
	return []*featurev1beta1.Constraint{constraint}
}

func syncGoCmd() *cobra.Command {
	var f string
	var repoPath string
	cmd := &cobra.Command{
		Use:   "go",
		Short: "sync go code to the repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			src, err := os.ReadFile(f)
			if err != nil {
				return err
			}
			fset := token.NewFileSet()
			pf, err := parser.ParseFile(fset, f, src, parser.ParseComments)
			if err != nil {
				return err
			}
			namespace := Namespace{}

			r, err := repo.NewLocal(repoPath, secrets.NewSecretsOrFail())
			if err != nil {
				return err
			}
			rootMD, _, err := r.ParseMetadata(ctx)
			if err != nil {
				return err
			}
			registry, err := r.BuildDynamicTypeRegistry(ctx, rootMD.ProtoDirectory)
			if err != nil {
				return err
			}

			ast.Inspect(pf, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.File:
					namespace.Name = x.Name.Name[5:]
				case *ast.FuncDecl:
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
					}
					return false
				}
				return true
			})
			// TODO static context
			fmt.Printf("\n\n%+v\n", namespace)

      // Now we need to write/merge it..
      nsExists := false
      for _, nsFromMeta := range rootMD.Namespaces {
        if namespace.Name == nsFromMeta {
          nsExists = true
          break
        }
      }
      if !nsExists {
        if err := r.AddNamespace(cmd.Context(), namespace.Name); err != nil {
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
      }

      return nil
    },
  }
  cmd.Flags().StringVarP(&f, "file", "f", "lekko.go", "file to sync") // TODO make this less dumb
  cmd.Flags().StringVarP(&repoPath, "path", "p", "", "path to the repo location")
  return cmd
}

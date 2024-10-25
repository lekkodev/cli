package gen

import (
	"fmt"
	"testing"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/lainio/err2/assert"
	protoutils "github.com/lekkodev/cli/pkg/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestThat(t *testing.T) {
	oauthStruct := Struct{
		Name: "OAuthDeviceConfig",
		Fields: []Field{
			{Name: "VerificationUri", Type: Type{Name: "String"}},
			{Name: "PollingIntervalSeconds", Type: Type{Name: "Int"}},
		},
	}

	getDeviceOauth := Function{
		Name: "getDeviceOauth",
		Arguments: []Argument{
			{Key: "env", Type: Type{Name: "String"}},
		},
		Return: Type{Name: "OAuthDeviceConfig"},
		Body: []Statement{
			ReturnStmt{
				Expression: ITE{
					Condition: BinaryExpr{
						Left:     Variable{Name: "env"},
						Operator: "=",
						Right:    Scalar{Value: "staging"},
					},
					Then: StructInstance{
						StructName: "OAuthDeviceConfig",
						Fields: []Expression{
							Scalar{Value: "https://app-staging.lekko.com/login/device"},
							Scalar{Value: 5},
						},
					},
					Else: ITE{
						Condition: BinaryExpr{
							Left:     Variable{Name: "env"},
							Operator: "=",
							Right:    Scalar{Value: "development"},
						},
						Then: StructInstance{
							StructName: "OAuthDeviceConfig",
							Fields: []Expression{
								Scalar{Value: "http://localhost:8080/login/device"},
								Scalar{Value: 5},
							},
						},
						Else: StructInstance{
							StructName: "OAuthDeviceConfig",
							Fields: []Expression{
								Scalar{Value: "https://app.lekko.com/login/device"},
								Scalar{Value: 5},
							},
						},
					},
				},
			},
		},
	}

	assertStmt := Assert{
		Expression: BinaryExpr{
			Left: BinaryExpr{
				Left:     Variable{Name: "(VerificationUri (getDeviceOauth \"staging\"))"},
				Operator: "=",
				Right:    Scalar{Value: "https://app-staging.lekko.com/login/device"},
			},
			Operator: "=",
			Right:    Scalar{Value: true},
		},
	}

	namespace := Namespace{
		Structs:   []Struct{oauthStruct},
		Functions: []Function{getDeviceOauth},
		Asserts:   []Assert{assertStmt},
	}

	fmt.Println(namespace.AsCode())
}

func TestGenSmtForFeature_IntType(t *testing.T) {
	typeRegistry, err := protoutils.FileDescriptorSetToTypeRegistry(protoutils.NewDefaultFileDescriptorSet())
	if err != nil {
		panic(err)
	}
	g := &smtGenerator{
		repoContents: &featurev1beta1.RepositoryContents{},
		TypeRegistry: typeRegistry,
	}

	v5, err := proto.Marshal(wrapperspb.Int64(5))
	if err != nil {
		panic(err)
	}
	v42, err := proto.Marshal(wrapperspb.Int64(42))
	if err != nil {
		panic(err)
	}
	feature := &featurev1beta1.Feature{
		Key:  "test_feature",
		Type: featurev1beta1.FeatureType_FEATURE_TYPE_INT,
		Tree: &featurev1beta1.Tree{
			DefaultNew: &featurev1beta1.Any{
				TypeUrl: "type.googleapis.com/google.protobuf.Int64Value",
				Value:   v5,
			},
			Constraints: []*featurev1beta1.Constraint{
				{
					ValueNew: &featurev1beta1.Any{
						TypeUrl: "type.googleapis.com/google.protobuf.Int64Value",
						Value:   v42,
					},
					RuleAstNew: &rulesv1beta3.Rule{
						Rule: &rulesv1beta3.Rule_Atom{
							Atom: &rulesv1beta3.Atom{
								ContextKey:         "user_age",
								ComparisonOperator: rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN,
								ComparisonValue: &structpb.Value{
									Kind: &structpb.Value_NumberValue{
										NumberValue: 18,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	smt, err := g.genSmtForFeature(feature)
	assert.NoError(err)
	fmt.Printf("%+v", smt.AsCode())
}

package gen

import (
	"fmt"
	"strings"

	featurev1beta1 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/feature/v1beta1"
	rulesv1beta3 "buf.build/gen/go/lekkodev/cli/protocolbuffers/go/lekko/rules/v1beta3"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
)

type Type struct {
	Name string
}

type Argument struct {
	Key  string
	Type Type
}

type Field struct {
	Name  string
	Type  Type
	Value interface{}
}

func (f Field) AsCode() string {
	return fmt.Sprintf("(%s %s)", f.Name, f.Type.Name)
}

type Struct struct {
	Name   string
	Fields []Field
}

func (s *Struct) AddField(field Field) {
	s.Fields = append(s.Fields, field)
}

func (s Struct) AsCode() string {
	fieldCodes := []string{}
	for _, field := range s.Fields {
		fieldCodes = append(fieldCodes, field.AsCode())
	}
	return fmt.Sprintf("(declare-datatype %s\n  ((mk%s \n    %s\n  ))\n)", s.Name, s.Name, strings.Join(fieldCodes, "\n    "))
}

type StructInstance struct {
	StructName string
	Fields     []Expression
}

func (si StructInstance) AsCode() string {
	fieldValues := []string{}
	for _, field := range si.Fields {
		fieldValues = append(fieldValues, field.AsCode())
	}
	return fmt.Sprintf("(mk%s %s)", si.StructName, strings.Join(fieldValues, " "))
}

func (si StructInstance) Evaluate() interface{} {
	return nil
}

type Pluck struct {
	Source    *Struct
	FieldName string
}

func (p Pluck) Evaluate() interface{} {
	for _, field := range p.Source.Fields {
		if field.Name == p.FieldName {
			return field.Value
		}
	}
	return errors.New("field not found")
}

type Scalar struct {
	Value interface{}
}

type Variable struct {
	Name  string
	Value Expression
	Type  Type
}

func (v Variable) AsCode() string {
	return v.Name
}

func (v Variable) Evaluate() interface{} {
	return v.Value
}

type BinaryExpr struct {
	Left     Expression
	Operator string
	Right    Expression
}

type Expression interface {
	Evaluate() interface{}
	AsCode() string
}

type Statement interface {
	Execute()
	AsCode() string
}

type Function struct {
	Name      string
	Arguments []Argument
	Body      []Statement
	Return    Type
}

type ReturnStmt struct {
	Expression Expression
}

func (r ReturnStmt) Evaluate() interface{} {
	return r.Expression.Evaluate()
}

func (r ReturnStmt) AsCode() string {
	return r.Expression.AsCode()
}

func (r ReturnStmt) Execute() {

}

type FunctionCall struct {
	FunctionName string
	Arguments    []Expression
}

func (f Function) Execute(arguments []interface{}) interface{} {
	if len(arguments) != len(f.Arguments) {
		fmt.Println("Argument mismatch")
		return nil
	}
	for _, stmt := range f.Body {
		stmt.Execute()
	}
	return nil
}

func (s Scalar) Evaluate() interface{} {
	return s.Value
}

func (b BinaryExpr) Evaluate() interface{} {
	leftVal := b.Left.Evaluate()
	rightVal := b.Right.Evaluate()

	switch b.Operator {
	case "+":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) + rightVal.(int)
		case float64:
			return leftVal.(float64) + rightVal.(float64)
		}
	case "-":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) - rightVal.(int)
		case float64:
			return leftVal.(float64) - rightVal.(float64)
		}
	case "*":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) * rightVal.(int)
		case float64:
			return leftVal.(float64) * rightVal.(float64)
		}
	case "/":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) / rightVal.(int)
		case float64:
			return leftVal.(float64) / rightVal.(float64)
		}

	// Comparison operators
	case "==":
		return leftVal == rightVal
	case "!=":
		return leftVal != rightVal
	case "<":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) < rightVal.(int)
		case float64:
			return leftVal.(float64) < rightVal.(float64)
		}
	case "<=":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) <= rightVal.(int)
		case float64:
			return leftVal.(float64) <= rightVal.(float64)
		}
	case ">":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) > rightVal.(int)
		case float64:
			return leftVal.(float64) > rightVal.(float64)
		}
	case ">=":
		switch leftVal.(type) {
		case int:
			return leftVal.(int) >= rightVal.(int)
		case float64:
			return leftVal.(float64) >= rightVal.(float64)
		}

	case "&&":
		leftBool, leftOk := leftVal.(bool)
		rightBool, rightOk := rightVal.(bool)
		if leftOk && rightOk {
			return leftBool && rightBool
		}
	case "||":
		leftBool, leftOk := leftVal.(bool)
		rightBool, rightOk := rightVal.(bool)
		if leftOk && rightOk {
			return leftBool || rightBool
		}
	}

	return nil
}

func (f FunctionCall) Execute() {
	// Function lookup logic should go here
	// Evaluate arguments
	// Execute the function
}

type ITE struct {
	Condition Expression
	Then      Expression
	Else      Expression
}

func (i ITE) AsCode() string {
	return fmt.Sprintf("(ite %s %s %s)", i.Condition.AsCode(), i.Then.AsCode(), i.Else.AsCode())
}

func (i ITE) Execute() {

}

func (i ITE) Evaluate() interface{} {
	conditionResult := i.Condition.Evaluate()
	if conditionResult.(bool) {
		return i.Then.Evaluate()
	} else {
		return i.Else.Evaluate()
	}
}

type Assert struct {
	Expression Expression
}

func (a Assert) AsCode() string {
	return fmt.Sprintf("(assert %s)", a.Expression.AsCode())
}

func (a Argument) AsCode() string {
	return fmt.Sprintf("(%s %s)", a.Key, a.Type.Name)
}

func (s Scalar) AsCode() string {
	switch v := s.Value.(type) {
	case string:
		return fmt.Sprintf("\"%s\"", v)
	default:
		return fmt.Sprintf("%v", s.Value)
	}
}

func (b BinaryExpr) AsCode() string {
	var smtOperator string
	switch b.Operator {
	case "+":
		smtOperator = "+"
	case "-":
		smtOperator = "-"
	case "*":
		smtOperator = "*"
	case "/":
		smtOperator = "/"
	case "==":
		smtOperator = "="
	case "<=":
		smtOperator = "<="
	case ">=":
		smtOperator = ">="
	case "<":
		smtOperator = "<"
	case ">":
		smtOperator = ">"
	case "&&":
		smtOperator = "and"
	case "||":
		smtOperator = "or"
	default:
		smtOperator = b.Operator
	}
	return fmt.Sprintf("(%s %s %s)", smtOperator, b.Left.AsCode(), b.Right.AsCode())

}

func (p Pluck) AsCode() string {
	return p.FieldName
}

func (f Function) AsCode() string {
	argList := []string{}
	for _, arg := range f.Arguments {
		argList = append(argList, arg.AsCode())
	}
	argumentsCode := strings.Join(argList, " ")

	bodyCode := ""
	for _, stmt := range f.Body {
		bodyCode += stmt.AsCode() + "\n"
	}

	return fmt.Sprintf("(define-fun %s (%s) %s\n  %s)", f.Name, argumentsCode, f.Return.Name, bodyCode)
}

type Contains struct {
	Container Expression
	Element   Expression
}

func (c Contains) AsCode() string {
	switch c.Container.(type) {
	case *Variable, *Scalar:
		return fmt.Sprintf("(str.contains %s %s)", c.Container.AsCode(), c.Element.AsCode())
	default:
		// need to either add this to SMT-LIB, fake it, or switch to cvc
		return fmt.Sprintf("(array-contains %s %s)", c.Container.AsCode(), c.Element.AsCode())
	}
}

type Namespace struct {
	Structs   []Struct
	Functions []Function
	Asserts   []Assert
}

func (ns Namespace) AsCode() string {
	code := ""

	for _, s := range ns.Structs {
		code += s.AsCode() + "\n"
	}

	for _, f := range ns.Functions {
		code += f.AsCode() + "\n"
	}

	for _, a := range ns.Asserts {
		code += a.AsCode() + "\n"
	}

	return code
}

type smtGenerator struct {
	repoContents *featurev1beta1.RepositoryContents
	TypeRegistry *protoregistry.Types
}

func (g *smtGenerator) genSmtForFeature(f *featurev1beta1.Feature) (*Function, error) {
	var ret = &Function{}
	var retType string
	switch f.Type {
	case featurev1beta1.FeatureType_FEATURE_TYPE_BOOL:
		retType = "Bool"
	case featurev1beta1.FeatureType_FEATURE_TYPE_INT:
		retType = "Int"
	case featurev1beta1.FeatureType_FEATURE_TYPE_FLOAT:
		retType = "Double"
	case featurev1beta1.FeatureType_FEATURE_TYPE_STRING:
		retType = "String"
	case featurev1beta1.FeatureType_FEATURE_TYPE_PROTO:
		panic("TODO")
	default:
		panic("TODO")
	}
	ret.Name = f.Key
	ret.Return = Type{Name: retType}

	var ites []*ITE
	for _, constraint := range f.Tree.Constraints {
		rule, err := g.translateRule(constraint.GetRuleAstNew())
		if err != nil {
			return nil, errors.Wrap(err, "rule")
		}
		value, err := g.translateAnyValue(constraint.ValueNew)
		if err != nil {
			return nil, err
		}
		rule.Then = Scalar{Value: value}
		ites = append(ites, rule)
	}
	defaultValue, err := g.translateAnyValue(f.Tree.DefaultNew)
	if err != nil {
		return nil, err
	}
	if len(ites) == 0 {
		ret.Body = append(ret.Body, ReturnStmt{Expression: Scalar{Value: defaultValue}})
	} else {
		for idx, ite := range ites {
			if idx == 0 {
				ret.Body = append(ret.Body, ReturnStmt{Expression: ite})
			} else {
				ites[idx-1].Else = ite
			}
		}
		ites[len(ites)-1].Else = Scalar{Value: defaultValue}
	}

	return ret, nil
}

func (g *smtGenerator) translateRule(rule *rulesv1beta3.Rule) (*ITE, error) {
	condition := BinaryExpr{}
	ret := &ITE{}
	if rule == nil {
		return nil, nil
	}
	switch v := rule.GetRule().(type) {
	case *rulesv1beta3.Rule_Atom:
		condition.Left = Variable{
			Name: v.Atom.ContextKey,
		}
		switch v.Atom.GetComparisonOperator() {
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_EQUALS:
			condition.Operator = "="
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_NOT_EQUALS:
			condition.Operator = "!="
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN:
			condition.Operator = "<"
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_LESS_THAN_OR_EQUALS:
			condition.Operator = "<="
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN:
			condition.Operator = ">"
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_GREATER_THAN_OR_EQUALS:
			condition.Operator = ">="
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINS:
			panic("unimplemented")
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_STARTS_WITH:
			panic("unimplemented")
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_ENDS_WITH:
			panic("unimplemented")
		case rulesv1beta3.ComparisonOperator_COMPARISON_OPERATOR_CONTAINED_WITHIN:
			panic("unimplemented")
		default:
			return nil, fmt.Errorf("unsupported operator %+v", v.Atom.ComparisonOperator)
		}
		condition.Right = Scalar{Value: v.Atom.ComparisonValue.AsInterface()}
	case *rulesv1beta3.Rule_Not:
		panic("TODO")
	case *rulesv1beta3.Rule_LogicalExpression:
		condition.Operator = "&&"
		switch v.LogicalExpression.GetLogicalOperator() {
		case rulesv1beta3.LogicalOperator_LOGICAL_OPERATOR_OR:
			condition.Operator = " || "
		}
		for _, rule := range v.LogicalExpression.Rules {
			ite, err := g.translateRule(rule)
			if err != nil {
				return nil, err
			}
			condition.Right = ite
		}
	default:
		return nil, fmt.Errorf("unsupported type of rule %+v", v)
	}
	ret.Condition = condition
	return ret, nil
}

func (g *smtGenerator) translateAnyValue(val *featurev1beta1.Any) (interface{}, error) {
	mt, err := g.TypeRegistry.FindMessageByURL(val.TypeUrl)
	if err != nil {
		return nil, errors.Wrapf(err, "find %s", val.TypeUrl)
	}
	msg := mt.New().Interface()
	err = proto.UnmarshalOptions{Resolver: g.TypeRegistry}.Unmarshal(val.Value, msg)
	if err != nil {
		return msg, err
	}
	switch v := msg.(type) {
	case interface{ GetValue() interface{} }: // TODO --- Not sure if this is a good idea
		return v.GetValue(), nil
	default:
		return msg.ProtoReflect().Interface(), nil
	}
}

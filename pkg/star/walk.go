package star

import (
	"fmt"
	"io/ioutil"

	"github.com/bazelbuild/buildtools/build"
	butils "github.com/bazelbuild/buildtools/buildifier/utils"
	"github.com/pkg/errors"
)

func Walk(filename string) error {
	parser := butils.GetParser(inputTypeAuto)
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return errors.Wrap(err, "read file")
	}
	file, err := parser(filename, data)
	if err != nil {
		return errors.Wrap(err, "parse")
	}
	// for _, expr := range file.Stmt {
	// assignExpr, ok := expr.(*build.AssignExpr)
	// if !ok {
	// 	continue
	// }
	// lhs, rhs := assignExpr.LHS, assignExpr.RHS

	// }
	// var f *feature.Feature
	build.Walk(file, func(x build.Expr, stk []build.Expr) {
		fmt.Printf("[%d] %T: %s\n", len(stk), x, build.FormatString(x))
		// if x.(type)
	})
	return nil
}

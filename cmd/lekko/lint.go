package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lainio/err2/try"
	"github.com/lekkodev/cli/pkg/dotlekko"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"regexp"
	"strings"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
)

var Analyzer = &analysis.Analyzer{
	Name:             "lekkolint",
	Doc:              "Documented Linter",
	Requires:         []*analysis.Analyzer{inspect.Analyzer},
	Run:              run,
	RunDespiteErrors: true,
}

var (
	funcNamePattern = regexp.MustCompile(`^[Gg]et[A-Za-z0-9]+$`)
)

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		if !strings.Contains(file.Name.Name, "lekko") || file.Name.Name == "lekko" {
			// skip anything that doesn't have a lekko package
			// skips the lekko package, since we create the client there
			// this needs to be slightly more sophisticated
			continue
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch x := node.(type) {
			case *ast.FuncDecl:
				checkFuncDecl(pass, x)
			case *ast.IfStmt:
				checkIfStmt(pass, x)
			case *ast.BinaryExpr:
				checkBinaryExpr(pass, x)
			}

			return true
		})
	}
	return nil, nil
}

func checkFuncDecl(pass *analysis.Pass, node *ast.FuncDecl) {
	if !funcNamePattern.MatchString(node.Name.Name) {
		pass.Reportf(node.Pos(), "Function names must be like 'getConfigName'.")
	}
	if node.Type.Results == nil {
		pass.Reportf(node.Pos(), "Functions must explicitly specify return types.")
	}
	if node.Body != nil {
		for _, stmt := range node.Body.List {
			switch stmt.(type) {
			case *ast.IfStmt, *ast.ReturnStmt:
				// Allowed statements
			default:
				pass.Reportf(stmt.Pos(), "Only if and return statements are allowed inside config functions.")
			}
		}
	}
}

func checkIfStmt(pass *analysis.Pass, node *ast.IfStmt) {
	if node.Body != nil {
		for _, stmt := range node.Body.List {
			if _, ok := stmt.(*ast.ReturnStmt); !ok {
				pass.Reportf(stmt.Pos(), "If statements may only contain return statements.")
			}
		}
	}
}

func checkBinaryExpr(pass *analysis.Pass, node *ast.BinaryExpr) {
	if _, ok := node.X.(*ast.BasicLit); ok {
		pass.Reportf(node.Pos(), "Literals must be on the right side of binary expressions.")
	}
	if _, ok := node.Y.(*ast.Ident); ok {
		pass.Reportf(node.Pos(), "Identifiers can't be on the right side.")
		y := node.Y
		x := node.X
		node.Y = x
		node.X = y
	}
}

func lintGoCmd() *cobra.Command {
	var lekkoPath string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "lint golang lekko configs",
		RunE: func(cmd *cobra.Command, args []string) error {
			//ctx := cmd.Context()
			if len(lekkoPath) == 0 {
				dot := try.To1(dotlekko.ReadDotLekko(""))
				lekkoPath = dot.LekkoPath
			}
			files, err := collectGoFiles(lekkoPath)
			if err != nil {
				return err
			}

			for _, file := range files {
				err := runAnalyzerOnFile(file)
				if err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&lekkoPath, "lekko-path", "p", "", "Path to Lekko native config files, will use autodetect if not set")
	return cmd
}

func collectGoFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func runAnalyzerOnFile(filePath string) error {
	if strings.HasSuffix(filePath, "_gen.go") {
		return nil
	}

	src, err := os.ReadFile(filePath)
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("open %s", filePath))
	}

	if bytes.Contains(src, []byte("<<<<<<<")) {
		return fmt.Errorf("%s has unresolved merge conflicts", filePath)
	}

	fset := token.NewFileSet()
	pf, err := parser.ParseFile(fset, filePath, src, parser.ParseComments)
	if err != nil {
		return err
	}

	pass := &analysis.Pass{
		Analyzer: Analyzer,
		Files:    []*ast.File{pf},
		Fset:     fset,
		Report:   func(d analysis.Diagnostic) { fmt.Printf("%s: %s\n", fset.Position(d.Pos), d.Message) },
	}

	_, err = Analyzer.Run(pass)
	if err != nil {
		return err
	}
	format.Node(os.Stdout, fset, pf)

	return nil
}

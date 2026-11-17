package main

import (
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const gofiPath = "github.com/michaelolof/gofi"

// checkHandlerTests reports a file under internal/http that declares a gofi
// handler and has no test file of the same name beside it: create.go is
// tested in create_test.go (TEST-15).
func checkHandlerTests(m *module) ([]problem, error) {
	var problems []problem

	for _, p := range m.code {
		rel, _ := m.relPkg(p.PkgPath)
		if !within(rel, "internal/http") || isTestPackage(rel, p.Name) {
			continue
		}

		for _, file := range p.Syntax {
			handler := firstHandler(p.TypesInfo, file)
			if handler == nil {
				continue
			}

			path := p.Fset.Position(file.Package).Filename
			testFile := strings.TrimSuffix(filepath.Base(path), ".go") + "_test.go"

			_, err := os.Stat(filepath.Join(filepath.Dir(path), testFile))
			if err == nil {
				continue
			}

			if !os.IsNotExist(err) {
				return nil, err
			}

			problems = append(problems, m.at(p.Fset, handler.Pos(), "%s has no test file: add %s beside it, with the handler's tests (TEST-15)", handler.Name, testFile))
		}
	}

	return problems, nil
}

// firstHandler returns the first package-level variable in file that is set
// to gofi.DefineHandler(...), or nil when file declares none.
func firstHandler(info *types.Info, file *ast.File) *ast.Ident {
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.VAR {
			continue
		}

		for _, spec := range gen.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			for i, value := range vs.Values {
				if i < len(vs.Names) && callsDefineHandler(info, value) {
					return vs.Names[i]
				}
			}
		}
	}

	return nil
}

func callsDefineHandler(info *types.Info, expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok {
		return false
	}

	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return false
	}

	fn, ok := info.Uses[sel.Sel].(*types.Func)

	return ok && fn.Pkg() != nil && fn.Pkg().Path() == gofiPath && fn.Name() == "DefineHandler"
}

// checkTestHomes reports a Go file whose build tag doesn't match its home
// (TEST-1): a contract test outside test/contract, a file in test/contract
// without the contract tag, and a file in test/flows without the integration
// tag. It reads the files themselves, because the build archcheck loads leaves
// contract tests out. Which _integration_test.go files carry the integration
// tag is for review.
func checkTestHomes(m *module) ([]problem, error) {
	var problems []problem

	err := filepath.WalkDir(m.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if path != m.root && outsideModule(path, d.Name()) {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(d.Name(), ".go") {
			return nil
		}

		expr, line, err := buildConstraint(path)
		if err != nil {
			return err
		}

		rel := m.rel(path)

		switch {
		case within(rel, "test/contract") && !requires(expr, "contract"):
			problems = append(problems, problem{path: rel, msg: "every file in test/contract has //go:build contract, so only task test:contract reaches the vendor (TEST-1)"})
		case !within(rel, "test/contract") && requires(expr, "contract"):
			problems = append(problems, problem{path: rel, line: line, msg: "a contract test lives in test/contract: move this file there (TEST-1)"})
		case within(rel, "test/flows") && !requires(expr, "integration"):
			problems = append(problems, problem{path: rel, msg: "every file in test/flows has //go:build integration, so a run without the test stack never builds a flow (TEST-1)"})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the test files' tags: %w", err)
	}

	return problems, nil
}

// outsideModule says whether the go tool leaves the folder out of the module:
// a name starting with a dot or an underscore, testdata, vendor, or a folder
// with its own go.mod, such as test/e2e (TEST-1).
func outsideModule(path, name string) bool {
	if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor" {
		return true
	}

	_, err := os.Stat(filepath.Join(path, "go.mod"))

	return err == nil
}

// buildConstraint returns the file's //go:build expression and its line, or
// nil when it has none.
func buildConstraint(path string) (constraint.Expr, int, error) {
	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly|parser.ParseComments)
	if err != nil {
		return nil, 0, err
	}

	for _, group := range file.Comments {
		if group.Pos() > file.Package {
			break
		}

		for _, c := range group.List {
			if !constraint.IsGoBuild(c.Text) {
				continue
			}

			expr, err := constraint.Parse(c.Text)
			if err != nil {
				return nil, 0, fmt.Errorf("%s: %w", path, err)
			}

			return expr, fset.Position(c.Pos()).Line, nil
		}
	}

	return nil, 0, nil
}

// requires says whether a file with expr is built only when tag is set:
// "contract" and "contract && linux" require contract, but "!contract" and
// "contract || linux" don't. It tries every setting of the expression's other
// tags with tag unset, and none may build the file.
func requires(expr constraint.Expr, tag string) bool {
	if expr == nil {
		return false
	}

	var others []string

	tagsIn(expr, func(t string) {
		if t != tag && !slices.Contains(others, t) {
			others = append(others, t)
		}
	})

	for setting := range 1 << len(others) {
		isSet := func(t string) bool {
			i := slices.Index(others, t)

			return i >= 0 && setting&(1<<i) != 0
		}

		if expr.Eval(isSet) {
			return false
		}
	}

	return true
}

// tagsIn calls found with each tag expr names.
func tagsIn(expr constraint.Expr, found func(string)) {
	switch e := expr.(type) {
	case *constraint.TagExpr:
		found(e.Tag)
	case *constraint.NotExpr:
		tagsIn(e.X, found)
	case *constraint.AndExpr:
		tagsIn(e.X, found)
		tagsIn(e.Y, found)
	case *constraint.OrExpr:
		tagsIn(e.X, found)
		tagsIn(e.Y, found)
	}
}

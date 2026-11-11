// Package errkinds checks where application error kinds are declared (ERR-5).
//
//   - Every errx.Kind is declared in internal/foundation/errkinds, as a package
//     variable. No other package declares a kind of its own.
//   - Its name is ErrKind<Name>, and its string is a unique snake_case literal
//     with no package prefix.
//
// tools/archcheck checks that each kind is created and checked somewhere. Whether
// that check is a branch, and not a choice of message, is for review.
package errkinds

import (
	"go/ast"
	"go/constant"
	"regexp"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks errx.Kind declarations.
var Analyzer = &analysis.Analyzer{
	Name: "errkinds",
	Doc:  "checks that every errx.Kind is declared once, in foundation/errkinds, as ErrKind<Name> = \"snake_case\"",
	Run:  run,
}

// home is the one package that declares kinds.
const home = "internal/foundation/errkinds"

var (
	kindName   = regexp.MustCompile(`^ErrKind[A-Z][A-Za-z0-9]*$`)
	kindString = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)
)

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	inHome := pkg.Rel == home
	declared := map[*ast.CallExpr]*ast.Ident{}
	strings := map[string]string{}

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Names) == len(vs.Values) {
						for i, value := range vs.Values {
							if call, ok := ast.Unparen(value).(*ast.CallExpr); ok {
								declared[call] = vs.Names[i]
							}
						}
					}
				}
			}
		}
	}

	for _, file := range where.Files(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !where.Calls(pass, call, where.Errx, "Kind") {
				return true
			}

			if !inHome {
				pass.Reportf(call.Pos(), "every application kind is declared in %s, and nowhere else: reuse a general kind, or add one there (ERR-5)", home)
				return true
			}

			checkDeclaration(pass, call, declared[call], strings)

			return true
		})
	}

	return nil, nil
}

func checkDeclaration(pass *analysis.Pass, call *ast.CallExpr, name *ast.Ident, seen map[string]string) {
	if name == nil {
		pass.Reportf(call.Pos(), "a kind is declared as a package variable: var ErrKind<Name> = errx.Kind(\"snake_case\") (ERR-5)")
		return
	}

	if !kindName.MatchString(name.Name) {
		pass.Reportf(name.Pos(), "a kind is named ErrKind<Name>: rename %s (ERR-5)", name.Name)
	}

	value := pass.TypesInfo.Types[call.Args[0]].Value
	if value == nil || value.Kind() != constant.String {
		pass.Reportf(call.Args[0].Pos(), "a kind's string is a literal, so the list shows every kind (ERR-5)")
		return
	}

	s := constant.StringVal(value)

	switch {
	case !kindString.MatchString(s):
		pass.Reportf(call.Args[0].Pos(), "a kind's string is snake_case: %q (ERR-5)", s)
	case seen[s] != "":
		pass.Reportf(call.Args[0].Pos(), "%s has the same string as %s: errx compares kinds by string (ERR-5)", name.Name, seen[s])
	default:
		seen[s] = name.Name
	}
}

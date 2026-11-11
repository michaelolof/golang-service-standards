// Package repos checks the shape of a repository package (DATA-1, DATA-2).
//
//   - internal/repository holds one package per aggregate, and nothing else:
//     no code in internal/repository itself, and no subpackages except tests.
//   - Each has repo.go, declaring Repo and New, and row.go.
//   - Repo is its only exported type. A schema type is unexported (DATA-2).
//   - Repo's exported methods take and return no SDK type (DATA-2). Other
//     layers never see how a record is stored. New may take the driver's handle.
//
// One column list and one scan function per table is for review.
package repos

import (
	"go/ast"
	"go/token"
	"path/filepath"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/repo"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks repository packages.
var Analyzer = &analysis.Analyzer{
	Name: "repos",
	Doc:  "checks that each repository package has repo.go and row.go, exports only Repo, and keeps SDK types out of its methods",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !where.Within(pkg.Rel, "internal/repository") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	if !repo.IsAggregate(pkg) {
		pass.Reportf(where.PackageClause(pass, "doc.go"), "internal/repository holds one package per aggregate, internal/repository/<aggregate>, and nothing else: move %s (DATA-1)", pkg.Rel)
		return nil, nil
	}

	checkFiles(pass)
	checkExports(pass)

	if named := repo.Type(pass); named != nil {
		for _, method := range repo.Methods(named) {
			if sdk := where.SDKIn(method.Type(), false); sdk != "" {
				pass.Reportf(method.Pos(), "Repo.%s takes or returns a type from %s: repository methods take and return domain types (DATA-2)", method.Name(), sdk)
			}
		}
	}

	return nil, nil
}

// checkFiles reports a missing repo.go or row.go, and a Repo or New outside repo.go.
func checkFiles(pass *analysis.Pass) {
	files := map[string]bool{}
	for _, file := range where.Files(pass) {
		files[where.FileName(pass, file)] = true
	}

	pos := where.PackageClause(pass, "repo.go")

	for _, name := range []string{"repo.go", "row.go"} {
		if !files[name] {
			pass.Reportf(pos, "a repository package has repo.go, with Repo and New, and row.go, with its column lists and scan functions: %s is missing (DATA-1)", name)
		}
	}

	for _, name := range []string{"Repo", "New"} {
		obj := pass.Pkg.Scope().Lookup(name)
		if obj == nil {
			pass.Reportf(pos, "a repository package declares %s in repo.go (DATA-1)", name)
			continue
		}

		if files["repo.go"] && filepath.Base(pass.Fset.Position(obj.Pos()).Filename) != "repo.go" {
			pass.Reportf(obj.Pos(), "%s belongs in repo.go (DATA-1)", name)
		}
	}
}

// checkExports reports exported types other than Repo.
func checkExports(pass *analysis.Pass) {
	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}

			for _, spec := range gen.Specs {
				ts, _ := spec.(*ast.TypeSpec)
				if ts.Name.IsExported() && ts.Name.Name != "Repo" {
					pass.Reportf(ts.Name.Pos(), "Repo is a repository's only exported type: %s goes in domain if callers need it, or becomes unexported in row.go (DATA-1, DATA-2)", ts.Name.Name)
				}
			}
		}
	}
}

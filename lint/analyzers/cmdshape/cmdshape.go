// Package cmdshape checks the shape of a binary's package main.
//
//   - main.go declares main and run, and nothing else (LAY-2).
//   - Each service in cmd/services has a wire.go holding one function,
//     func wire(ctx context.Context, cfg <role>Config) (*app, func(), error) (LAY-3).
//
// gocritic's exitAfterDefer and revive's deep-exit already check how main exits.
package cmdshape

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks main.go and wire.go.
var Analyzer = &analysis.Analyzer{
	Name: "cmdshape",
	Doc:  "checks that main.go holds only main and run, and that each service has one wire function in wire.go",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || pkg.Name != "main" || !where.Within(pkg.Rel, "cmd") {
		return nil, nil
	}

	var wireFile *ast.File

	for _, file := range where.Files(pass) {
		switch where.FileName(pass, file) {
		case "main.go":
			checkMain(pass, file)
		case "wire.go":
			wireFile = file
		}
	}

	if !where.Within(pkg.Rel, "cmd/services") {
		return nil, nil
	}

	if wireFile == nil {
		pass.Reportf(where.PackageClause(pass, "main.go"), "%s has no wire.go: one wire function builds every dependency at startup (LAY-3)", pkg.Rel)
		return nil, nil
	}

	checkWire(pass, wireFile)

	return nil, nil
}

// checkMain reports every declaration in main.go other than main and run.
func checkMain(pass *analysis.Pass, file *ast.File) {
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil && (d.Name.Name == "main" || d.Name.Name == "run") {
				continue
			}

			pass.Reportf(d.Name.Pos(), "main.go declares only main and run: move %s to the file named for it, such as config.go or wire.go (LAY-2)", d.Name.Name)
		case *ast.GenDecl:
			if d.Tok == token.IMPORT {
				continue
			}

			pass.Reportf(d.Pos(), "main.go declares only main and run: move this %s to the file named for it, such as config.go or wire.go (LAY-2)", d.Tok)
		}
	}
}

// checkWire reports a wire.go whose functions aren't exactly one wire with the
// standard signature. Types, such as app, may sit beside it.
func checkWire(pass *analysis.Pass, file *ast.File) {
	found := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fn.Recv != nil || fn.Name.Name != "wire" {
			pass.Reportf(fn.Name.Pos(), "wire.go holds one function, wire: move %s out (LAY-3)", fn.Name.Name)
			continue
		}

		found = true

		if sig, ok := pass.TypesInfo.Defs[fn.Name].Type().(*types.Signature); ok && !wireShape(sig) {
			pass.Reportf(fn.Name.Pos(), "wire's signature is func wire(ctx context.Context, cfg <role>Config) (*app, func(), error) (LAY-3)")
		}
	}

	if !found {
		pass.Reportf(file.Name.Pos(), "wire.go has no wire function: func wire(ctx context.Context, cfg <role>Config) (*app, func(), error) (LAY-3)")
	}
}

// wireShape says whether sig is (context.Context, T) (*U, func(), error).
func wireShape(sig *types.Signature) bool {
	params, results := sig.Params(), sig.Results()
	if params.Len() != 2 || results.Len() != 3 {
		return false
	}

	if !where.IsContext(params.At(0).Type()) {
		return false
	}

	if _, ok := results.At(0).Type().(*types.Pointer); !ok {
		return false
	}

	cleanup, ok := results.At(1).Type().(*types.Signature)
	if !ok || cleanup.Params().Len() != 0 || cleanup.Results().Len() != 0 {
		return false
	}

	return where.IsError(results.At(2).Type())
}

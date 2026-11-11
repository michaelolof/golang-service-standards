// Package adapters checks an adapter package's API (ABS-7). In a package under
// internal/adapters:
//
//   - New and Open take the package's own Config, and a ctx first only when
//     their body uses it: New(cfg Config) (*T, error). Role constructors such as
//     redis.NewLock(client, ttl) are left alone.
//   - No exported function, method, field or type uses a type from an SDK the
//     standard keeps at the edge. A store adapter (one named <name>store) may
//     hand its database driver's types to repositories, which import that
//     driver too (LAY-8).
//
// Whether New does network I/O, and whether every SDK error is translated, is
// for review.
package adapters

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks adapter constructors and exported types.
var Analyzer = &analysis.Analyzer{
	Name: "adapters",
	Doc:  "checks that adapters are built by New(cfg) (*T, error) and leak no SDK type through their exported API",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	layer, item := where.Layer(pkg.Rel)
	if layer != "adapters" || item == "" {
		return nil, nil
	}

	c := checker{pass: pass, store: strings.HasSuffix(item, "store")}

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				c.checkFunc(d)
			case *ast.GenDecl:
				c.checkGenDecl(d)
			}
		}
	}

	return nil, nil
}

type checker struct {
	pass  *analysis.Pass
	store bool
}

func (c checker) checkFunc(fn *ast.FuncDecl) {
	obj, ok := c.pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok || !fn.Name.IsExported() {
		return
	}

	if fn.Recv == nil && (fn.Name.Name == "New" || fn.Name.Name == "Open") {
		c.checkConstructor(fn, obj.Signature())
	}

	if recv := obj.Signature().Recv(); recv != nil && !ast.IsExported(where.TypeName(recv.Type())) {
		return // a method on an unexported type isn't part of the API
	}

	if sdk := c.leak(obj.Signature()); sdk != "" {
		c.pass.Reportf(fn.Name.Pos(), "%s exposes a type from %s: an adapter wraps its SDK and leaks no SDK type (ABS-7)", fn.Name.Name, sdk)
	}
}

// checkConstructor reports a New or Open that isn't (Config) or (ctx, Config)
// returning (*T, error), or that takes a ctx it never uses.
func (c checker) checkConstructor(fn *ast.FuncDecl, sig *types.Signature) {
	params := sig.Params()
	withCtx := params.Len() == 2 && where.IsContext(params.At(0).Type())

	var cfg types.Type

	switch {
	case params.Len() == 1:
		cfg = params.At(0).Type()
	case withCtx:
		cfg = params.At(1).Type()
	}

	results := sig.Results()
	shaped := results.Len() == 2 && where.IsError(results.At(1).Type())

	if shaped {
		_, shaped = results.At(0).Type().(*types.Pointer)
	}

	if cfg == nil || !c.ownConfig(cfg) || !shaped {
		c.pass.Reportf(fn.Name.Pos(), "an adapter is built by %s(cfg Config) (*T, error), taking its own Config struct (ABS-7)", fn.Name.Name)
		return
	}

	if withCtx && !uses(c.pass, fn.Body, params.At(0)) {
		c.pass.Reportf(fn.Name.Pos(), "%s takes a ctx only when building the client needs one, and this one is never used (ABS-7)", fn.Name.Name)
	}
}

// ownConfig says whether t is a struct declared in this package whose name ends in Config.
func (c checker) ownConfig(t types.Type) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() != c.pass.Pkg || !strings.HasSuffix(named.Obj().Name(), "Config") {
		return false
	}

	_, ok = named.Underlying().(*types.Struct)

	return ok
}

func (c checker) checkGenDecl(decl *ast.GenDecl) {
	for _, spec := range decl.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			c.checkTypeSpec(s)
		case *ast.ValueSpec:
			if decl.Tok == token.IMPORT {
				continue
			}

			for _, name := range s.Names {
				if obj := c.pass.TypesInfo.Defs[name]; name.IsExported() && obj != nil {
					if sdk := c.leak(obj.Type()); sdk != "" {
						c.pass.Reportf(name.Pos(), "%s has a type from %s: an adapter leaks no SDK type (ABS-7)", name.Name, sdk)
					}
				}
			}
		}
	}
}

func (c checker) checkTypeSpec(spec *ast.TypeSpec) {
	if !spec.Name.IsExported() {
		return
	}

	obj, ok := c.pass.TypesInfo.Defs[spec.Name].(*types.TypeName)
	if !ok {
		return
	}

	if st, ok := obj.Type().Underlying().(*types.Struct); ok && !obj.IsAlias() {
		for field := range st.Fields() {
			if !field.Exported() {
				continue
			}

			if sdk := c.leak(field.Type()); sdk != "" {
				c.pass.Reportf(field.Pos(), "%s.%s has a type from %s: keep the SDK's types unexported (ABS-7)", spec.Name.Name, field.Name(), sdk)
			}
		}

		return
	}

	t := obj.Type()
	if !obj.IsAlias() {
		t = t.Underlying()
	}

	if sdk := c.leak(t); sdk != "" {
		c.pass.Reportf(spec.Name.Pos(), "%s is built from a type in %s: an adapter leaks no SDK type (ABS-7)", spec.Name.Name, sdk)
	}
}

// leak returns the SDK path of the first SDK type t mentions, or "".
func (c checker) leak(t types.Type) string {
	return where.SDKIn(t, c.store)
}

// uses says whether body refers to v.
func uses(pass *analysis.Pass, body *ast.BlockStmt, v *types.Var) bool {
	if body == nil {
		return false
	}

	found := false

	ast.Inspect(body, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && pass.TypesInfo.Uses[ident] == v {
			found = true
		}

		return !found
	})

	return found
}

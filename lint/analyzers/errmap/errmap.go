// Package errmap checks what a gofi handler returns (ERR-7). Under
// internal/http, every error a handler returns is one of:
//
//   - nil;
//   - a call into the project's httperr package, such as
//     httperr.Map(stamp, err).WithMsg("api key not found");
//   - a gofi.Context method, such as c.Send(200, s.Ok) or c.Next(), whose error
//     goes back unchanged (ERR-3);
//   - the error of a handler helper: an internal/http function that takes the
//     gofi.Context and returns an error, such as GetEvent(c, id), and isn't
//     shaped like middleware, func(gofi.Context) error. In a package that
//     declares handlers, a helper's own returns are checked the same way;
//   - any of these wrapped once more with errx.Wrap, which wrapcheck asks for
//     when the helper is in another package. The encoder still finds the
//     *httperr inside.
//
// So an error from core, or from ValidateAndBind, never reaches the encoder
// unmapped. Middleware isn't checked. Whether a .WithMsg follows a check of the
// kind is for review.
package errmap

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks handler returns.
var Analyzer = &analysis.Analyzer{
	Name: "errmap",
	Doc:  "checks that a gofi handler returns its errors through httperr",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !where.Within(pkg.Rel, "internal/http") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	c := checker{pass: pass, pkg: pkg}
	funcs := map[types.Object]*ast.FuncDecl{}
	handlers := 0

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				funcs[pass.TypesInfo.Defs[fn.Name]] = fn
			}
		}
	}

	for _, file := range where.Files(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if ok && len(call.Args) == 1 && where.Calls(pass, call, where.Gofi, "DefineHandler") {
				handlers++
				c.checkHandler(call.Args[0], funcs)
			}

			return true
		})
	}

	if handlers == 0 {
		return nil, nil
	}

	for obj, fn := range funcs {
		if f, ok := obj.(*types.Func); ok && c.isHelper(f) {
			c.checkReturns(fn.Body, f.Signature())
		}
	}

	return nil, nil
}

type checker struct {
	pass *analysis.Pass
	pkg  where.Package
}

// checkHandler checks the Handler field of a RouteOptions literal: a function
// literal, or a function declared in this package.
func (c checker) checkHandler(expr ast.Expr, funcs map[types.Object]*ast.FuncDecl) {
	lit, ok := ast.Unparen(expr).(*ast.CompositeLit)
	if !ok {
		return
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if key, isIdent := kv.Key.(*ast.Ident); !ok || !isIdent || key.Name != "Handler" {
			continue
		}

		sig, _ := c.pass.TypesInfo.TypeOf(kv.Value).(*types.Signature)
		if sig == nil {
			return
		}

		switch fn := ast.Unparen(kv.Value).(type) {
		case *ast.FuncLit:
			c.checkReturns(fn.Body, sig)
		case *ast.Ident:
			if decl := funcs[c.pass.TypesInfo.Uses[fn]]; decl != nil {
				c.checkReturns(decl.Body, sig)
			}
		}
	}
}

// isHelper says whether fn is a handler helper: it takes the gofi.Context and
// returns an error, and isn't shaped like middleware, func(gofi.Context) error.
func (c checker) isHelper(fn *types.Func) bool {
	sig := fn.Signature()
	if sig.Params().Len() == 1 && sig.Results().Len() == 1 {
		return false
	}

	takesContext, returnsError := false, false

	for v := range sig.Params().Variables() {
		takesContext = takesContext || where.IsNamed(v.Type(), where.Gofi, "Context")
	}

	for v := range sig.Results().Variables() {
		returnsError = returnsError || where.IsError(v.Type())
	}

	return takesContext && returnsError
}

// checkReturns reports each error body returns, outside nested closures, that
// isn't mapped.
func (c checker) checkReturns(body *ast.BlockStmt, sig *types.Signature) {
	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			for _, result := range errorResults(node, sig) {
				if !c.mapped(body, result, node.Pos()) {
					c.pass.Reportf(result.Pos(), "a handler returns its errors through httperr: return httperr.Map(stamp, err) (ERR-7)")
				}
			}
		}

		return true
	})
}

// errorResults returns the expressions ret returns in sig's error results. A
// single call that returns them all, return f(), is returned whole.
func errorResults(ret *ast.ReturnStmt, sig *types.Signature) []ast.Expr {
	results := sig.Results()

	if len(ret.Results) == 1 && results.Len() > 1 {
		return ret.Results
	}

	var exprs []ast.Expr

	for i, expr := range ret.Results {
		if i < results.Len() && where.IsError(results.At(i).Type()) {
			exprs = append(exprs, expr)
		}
	}

	return exprs
}

// mapped says whether expr is nil, a call chain rooted in httperr, a gofi.Context
// method, a call to a handler helper, or a variable last set from one of those
// calls before pos.
func (c checker) mapped(body *ast.BlockStmt, expr ast.Expr, pos token.Pos) bool {
	if c.pass.TypesInfo.Types[expr].IsNil() {
		return true
	}

	if ident, ok := ast.Unparen(expr).(*ast.Ident); ok {
		call := latestCall(c.pass, body, ident, pos)
		return call != nil && c.mappedCall(call)
	}

	call, ok := ast.Unparen(expr).(*ast.CallExpr)

	return ok && c.mappedCall(call)
}

func (c checker) mappedCall(call *ast.CallExpr) bool {
	for {
		fn := where.Callee(c.pass, call)
		if fn == nil {
			return false
		}

		if fn.Pkg() != nil && (fn.Pkg().Name() == "httperr" || strings.HasSuffix(fn.Pkg().Path(), "/httperr")) {
			return true
		}

		if fn.Signature().Recv() != nil && fn.Pkg() != nil && fn.Pkg().Path() == where.Gofi {
			return true // c.Send, c.Next: their errors go back unchanged
		}

		if c.inHTTP(fn) && c.isHelper(fn) {
			return true // a helper's own returns are checked
		}

		if where.IsFunc(fn, where.Errx, "Wrap") && len(call.Args) == 2 {
			return c.mappedArg(call.Args[1])
		}

		sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok {
			return false
		}

		inner, ok := ast.Unparen(sel.X).(*ast.CallExpr) // a chained method: Map(...).WithMsg(...)
		if !ok {
			return false
		}

		call = inner
	}
}

// mappedArg says whether the error errx.Wrap wraps is mapped: a mapped call, or
// a variable last set from one. It reads the enclosing function's body.
func (c checker) mappedArg(arg ast.Expr) bool {
	if call, ok := ast.Unparen(arg).(*ast.CallExpr); ok {
		return c.mappedCall(call)
	}

	ident, ok := ast.Unparen(arg).(*ast.Ident)
	if !ok {
		return false
	}

	body := enclosingBody(c.pass, ident.Pos())
	if body == nil {
		return false
	}

	call := latestCall(c.pass, body, ident, ident.Pos())

	return call != nil && c.mappedCall(call)
}

// enclosingBody returns the body of the innermost function around pos.
func enclosingBody(pass *analysis.Pass, pos token.Pos) *ast.BlockStmt {
	file := where.FileOf(pass, pos)
	if file == nil {
		return nil
	}

	var body *ast.BlockStmt

	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil || pos < n.Pos() || pos >= n.End() {
			return false
		}

		switch fn := n.(type) {
		case *ast.FuncDecl:
			body = fn.Body
		case *ast.FuncLit:
			body = fn.Body
		}

		return true
	})

	return body
}

func (c checker) inHTTP(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}

	rel, ok := c.pkg.Import(fn.Pkg().Path())

	return ok && where.Within(rel, "internal/http")
}

// latestCall returns the call that last set ident before pos, reading body in
// source order, or nil when that value didn't come from a call.
func latestCall(pass *analysis.Pass, body *ast.BlockStmt, ident *ast.Ident, pos token.Pos) *ast.CallExpr {
	target := pass.TypesInfo.ObjectOf(ident)
	if target == nil {
		return nil
	}

	var latest ast.Expr

	ast.Inspect(body, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok || assign.Pos() > pos {
			return true
		}

		for i, lhs := range assign.Lhs {
			name, ok := lhs.(*ast.Ident)
			if !ok || pass.TypesInfo.ObjectOf(name) != target {
				continue
			}

			if len(assign.Lhs) == len(assign.Rhs) {
				latest = assign.Rhs[i]
			} else {
				latest = assign.Rhs[0]
			}
		}

		return true
	})

	call, _ := ast.Unparen(latest).(*ast.CallExpr)

	return call
}

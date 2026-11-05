// Package wrapgaps reports the unwrapped errors wrapcheck can't see.
//
// wrapcheck skips two forms. It stops at a closure when the return is a direct
// call, and it only recognises a call written pkg.F(), so pkg.F[T]() is never
// reported, in any function:
//
//	func() error { return repo.Save(ctx) }  // a return in a closure
//	s, err := gofi.ValidateAndBind[T](c)    // a generic call, with an explicit type
//	return err
//
// wrapgaps reports exactly those two forms, and nothing wrapcheck already
// reports, so an error never gets two findings. It decides whether a call
// needs wrapping the way wrapcheck does: the call is to another package or to
// an exported interface method, and its signature isn't on the ignore list.
package wrapgaps

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"github.com/gobwas/glob"
	"github.com/tomarrell/wrapcheck/v2/wrapcheck"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Settings configures the analyzer. golangci-lint decodes it from the linter's
// settings block. The fields mean what wrapcheck's settings of the same name
// mean, and wrapcheck's default ignore list always applies.
type Settings struct {
	// ExtraIgnoreSigs lists substrings of a signature. A call whose signature
	// contains one doesn't need wrapping.
	ExtraIgnoreSigs []string `json:"extra-ignore-sigs"`

	// IgnorePackageGlobs lists globs of package paths. A call into a matching
	// package doesn't need wrapping.
	IgnorePackageGlobs []string `json:"ignore-package-globs"`
}

// New returns the analyzer configured by s.
func New(s Settings) (*analysis.Analyzer, error) {
	globs := make([]glob.Glob, 0, len(s.IgnorePackageGlobs))

	for _, pattern := range s.IgnorePackageGlobs {
		g, err := glob.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("wrapgaps: ignore-package-globs %q: %w", pattern, err)
		}

		globs = append(globs, g)
	}

	c := checker{
		ignoreSigs: slices.Concat(wrapcheck.DefaultIgnoreSigs, s.ExtraIgnoreSigs),
		ignorePkgs: globs,
		errorType:  types.Universe.Lookup("error").Type(),
	}

	return &analysis.Analyzer{
		Name:     "wrapgaps",
		Doc:      "reports unwrapped errors returned in the forms wrapcheck skips: direct returns in closures, and generic calls",
		Requires: []*analysis.Analyzer{inspect.Analyzer},
		Run:      c.run,
	}, nil
}

type checker struct {
	ignoreSigs []string
	ignorePkgs []glob.Glob
	errorType  types.Type
}

func (c checker) run(pass *analysis.Pass) (any, error) {
	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	filter := []ast.Node{(*ast.ReturnStmt)(nil)}

	insp.WithStack(filter, func(n ast.Node, push bool, stack []ast.Node) bool {
		if ret, ok := n.(*ast.ReturnStmt); ok && push {
			c.check(pass, ret, stack)
		}

		return true
	})

	return nil, nil
}

// check reports the results of ret that wrapcheck skips. stack holds ret's
// ancestors, with ret last.
func (c checker) check(pass *analysis.Pass, ret *ast.ReturnStmt, stack []ast.Node) {
	for _, result := range ret.Results {
		switch expr := ast.Unparen(result).(type) {
		case *ast.CallExpr:
			if !c.returnsError(pass, expr) {
				continue
			}

			if isGenericCall(pass, expr) {
				c.report(pass, expr, expr.Pos(), "a generic function")
			} else if insideClosure(stack) {
				c.report(pass, expr, expr.Pos(), "a closure")
			}
		case *ast.Ident:
			if !types.Identical(typeOf(pass, expr), c.errorType) {
				continue
			}

			// wrapcheck follows err back to its call, except when that call is generic.
			if call := latestAssignment(pass, stack[0], expr); call != nil && isGenericCall(pass, call) {
				c.report(pass, call, expr.Pos(), "a generic function")
			}
		}
	}
}

// report reports call at pos, unless it doesn't need wrapping.
func (c checker) report(pass *analysis.Pass, call *ast.CallExpr, pos token.Pos, where string) {
	sel := calledSelector(call)
	if sel == nil {
		return // a call within the package, by name
	}

	fn := pass.TypesInfo.ObjectOf(sel.Sel)
	if fn == nil || fn.Pkg() == nil {
		return
	}

	sig := fn.String()
	if c.ignored(sig, fn.Pkg().Path()) {
		return
	}

	onInterface := sel.Sel.IsExported() && types.IsInterface(typeOf(pass, sel.X))
	if !onInterface && fn.Pkg() == pass.Pkg {
		return
	}

	pass.Reportf(pos, "error returned from %s should be wrapped (ERR-3): sig: %s", where, sig)
}

func (c checker) ignored(sig, pkgPath string) bool {
	for _, s := range c.ignoreSigs {
		if strings.Contains(sig, s) {
			return true
		}
	}

	for _, g := range c.ignorePkgs {
		if g.Match(pkgPath) {
			return true
		}
	}

	return false
}

// returnsError says whether call's result is an error, alone or in a tuple.
func (c checker) returnsError(pass *analysis.Pass, call *ast.CallExpr) bool {
	if tv, ok := pass.TypesInfo.Types[call.Fun]; ok && tv.IsType() {
		return false // a conversion
	}

	typ := pass.TypesInfo.TypeOf(call)

	tuple, ok := typ.(*types.Tuple)
	if !ok {
		return typ != nil && types.Identical(typ, c.errorType)
	}

	for v := range tuple.Variables() {
		if types.Identical(v.Type(), c.errorType) {
			return true
		}
	}

	return false
}

func typeOf(pass *analysis.Pass, expr ast.Expr) types.Type {
	if typ := pass.TypesInfo.TypeOf(expr); typ != nil {
		return typ
	}

	return types.Typ[types.Invalid]
}

func insideClosure(stack []ast.Node) bool {
	return slices.ContainsFunc(stack, func(n ast.Node) bool {
		_, ok := n.(*ast.FuncLit)
		return ok
	})
}

// isGenericCall says whether call names its type arguments, as in pkg.F[T]().
// A generic call whose types are inferred is written pkg.F(), and wrapcheck
// sees it.
func isGenericCall(pass *analysis.Pass, call *ast.CallExpr) bool {
	var named ast.Expr

	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.IndexExpr:
		named = fun.X
	case *ast.IndexListExpr:
		named = fun.X
	default:
		return false
	}

	// fns[0]() is also written with an index, but nothing is instantiated.
	switch fn := ast.Unparen(named).(type) {
	case *ast.Ident:
		_, ok := pass.TypesInfo.Instances[fn]
		return ok
	case *ast.SelectorExpr:
		_, ok := pass.TypesInfo.Instances[fn.Sel]
		return ok
	}

	return false
}

// calledSelector returns the x.F that call calls, or nil if it isn't written that way.
func calledSelector(call *ast.CallExpr) *ast.SelectorExpr {
	fun := ast.Unparen(call.Fun)

	switch f := fun.(type) {
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}

	sel, _ := ast.Unparen(fun).(*ast.SelectorExpr)

	return sel
}

// latestAssignment returns the call that last set ident before it was used,
// reading the source in order as wrapcheck does. It returns nil if the last
// assignment isn't a call.
func latestAssignment(pass *analysis.Pass, root ast.Node, ident *ast.Ident) *ast.CallExpr {
	target := pass.TypesInfo.ObjectOf(ident)
	if target == nil {
		return nil
	}

	var latest ast.Expr

	assigned := func(lhs []*ast.Ident, rhs []ast.Expr) {
		for i, name := range lhs {
			if name == nil || pass.TypesInfo.ObjectOf(name) != target || name.Pos() > ident.Pos() || len(rhs) == 0 {
				continue
			}

			if len(lhs) == len(rhs) {
				latest = rhs[i]
			} else {
				latest = rhs[0]
			}
		}
	}

	ast.Inspect(root, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			lhs := make([]*ast.Ident, len(node.Lhs))
			for i, expr := range node.Lhs {
				lhs[i], _ = expr.(*ast.Ident)
			}

			assigned(lhs, node.Rhs)
		case *ast.ValueSpec:
			assigned(node.Names, node.Values)
		}

		return true
	})

	if latest == nil {
		return nil
	}

	call, _ := ast.Unparen(latest).(*ast.CallExpr)

	return call
}

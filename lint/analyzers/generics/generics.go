// Package generics reports a type switch on a type parameter (ABS-8). Generic
// code is for logic that doesn't depend on T, so the behaviour that varies is
// passed in as a value instead:
//
//	switch any(value).(type) { // value has type T
//	case DeliveryReceiptJob:
package generics

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer reports type switches on type parameters.
var Analyzer = &analysis.Analyzer{
	Name:     "generics",
	Doc:      "reports a type switch on a value of a type parameter",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.TypeSwitchStmt)(nil)}, func(n ast.Node) {
		sw, _ := n.(*ast.TypeSwitchStmt)

		var assert *ast.TypeAssertExpr

		switch s := sw.Assign.(type) {
		case *ast.ExprStmt:
			assert, _ = s.X.(*ast.TypeAssertExpr)
		case *ast.AssignStmt:
			if len(s.Rhs) == 1 {
				assert, _ = s.Rhs[0].(*ast.TypeAssertExpr)
			}
		}

		if assert != nil && ofTypeParam(pass, assert.X) {
			pass.Reportf(sw.Pos(), "don't type-switch on a type parameter: pass the behaviour that varies in as a value (ABS-8)")
		}
	})

	return nil, nil
}

// ofTypeParam says whether expr converts a value of a type parameter to an
// interface, as any(v) does.
func ofTypeParam(pass *analysis.Pass, expr ast.Expr) bool {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}

	if tv, ok := pass.TypesInfo.Types[call.Fun]; !ok || !tv.IsType() {
		return false
	}

	t := pass.TypesInfo.TypeOf(call.Args[0])
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	_, ok = t.(*types.TypeParam)

	return ok
}

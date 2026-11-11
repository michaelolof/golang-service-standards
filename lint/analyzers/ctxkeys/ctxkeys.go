// Package ctxkeys checks context values (CTX-2).
//
//   - context.WithValue appears only in a file named context.go, beside the
//     WithX and XFrom accessors.
//   - The key passed to context.WithValue or ctx.Value is an unexported
//     struct{} type declared in the same package. revive's context-keys-type
//     already reports a key of a basic type, such as a string.
package ctxkeys

import (
	"go/ast"
	"go/types"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer checks context keys and where values are stored.
var Analyzer = &analysis.Analyzer{
	Name:     "ctxkeys",
	Doc:      "checks that context.WithValue is used only in context.go, with unexported struct{} keys",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (any, error) {
	if pkg, ok := where.Of(pass); ok && where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call, _ := n.(*ast.CallExpr)
		file := where.FileOf(pass, call.Pos())

		if file == nil || where.FileName(pass, file) == "context_test.go" {
			return
		}

		switch fn := where.Callee(pass, call); {
		case where.IsFunc(fn, "context", "WithValue") && len(call.Args) == 3:
			if where.FileName(pass, file) != "context.go" {
				pass.Reportf(call.Pos(), "context.WithValue appears only in context.go, inside a WithX accessor (CTX-2)")
			}

			checkKey(pass, call.Args[1])
		case isValueMethod(fn) && len(call.Args) == 1:
			checkKey(pass, call.Args[0])
		}
	})

	return nil, nil
}

// isValueMethod says whether fn is context.Context's Value method.
func isValueMethod(fn *types.Func) bool {
	if fn == nil || fn.Name() != "Value" {
		return false
	}

	recv := fn.Signature().Recv()

	return recv != nil && where.IsContext(recv.Type())
}

func checkKey(pass *analysis.Pass, key ast.Expr) {
	t := pass.TypesInfo.TypeOf(key)
	if t == nil {
		return
	}

	if _, basic := t.Underlying().(*types.Basic); basic {
		return // revive's context-keys-type reports it
	}

	named, ok := types.Unalias(t).(*types.Named)
	if ok && named.Obj().Pkg() == pass.Pkg && !named.Obj().Exported() {
		if st, ok := named.Underlying().(*types.Struct); ok && st.NumFields() == 0 {
			return
		}
	}

	pass.Reportf(key.Pos(), "a context key is an unexported struct{} type declared beside its accessors: type xKey struct{} (CTX-2)")
}

// Package goroutines reports a go statement outside internal/foundation/supervise
// and pkg/ libraries (CON-1, CON-2). Every other goroutine has an owner that
// waits for it: an errgroup with SetLimit inside one batch, a component under
// supervise.Run for a long-lived loop, or a durable job for work requested by a
// request or by core.
package goroutines

import (
	"go/ast"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer reports raw go statements.
var Analyzer = &analysis.Analyzer{
	Name: "goroutines",
	Doc:  "reports go statements outside foundation/supervise and pkg/",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.Within(pkg.Rel, "internal/foundation/supervise") || where.Within(pkg.Rel, "pkg") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	for _, file := range where.Files(pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			if stmt, ok := n.(*ast.GoStmt); ok {
				pass.Reportf(stmt.Pos(), "a goroutine needs an owner that waits for it: use an errgroup with SetLimit, a component under supervise.Run, or a durable job (CON-1, CON-2)")
			}

			return true
		})
	}

	return nil, nil
}

// Package jobs checks job handlers and job kinds (JOB-1, ERR-8).
//
//   - A job handler's Handle method, under internal/workers, never inspects an
//     error with errx.IsKind, errors.Is or errors.As: it returns the error, and
//     the runtime's one mapping picks the outcome. nilerr already reports a
//     handler that drops an error with return nil.
//   - The handler type is named for its action, not Execute, Process, Worker
//     or Processor.
//   - A job kind declared in internal/jobs is "<capability>.<action>", such as
//     "transfer.initiate".
//
// Whether Handle makes one call is for review.
package jobs

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks job handlers and kinds.
var Analyzer = &analysis.Analyzer{
	Name: "jobs",
	Doc:  "checks that job handlers are named for their action and never branch on an error, and that job kinds are capability.action",
	Run:  run,
}

var (
	vagueNames = map[string]bool{"Execute": true, "Process": true, "Processor": true, "Worker": true}
	kindValue  = regexp.MustCompile(`^[a-z][a-z0-9]*\.[a-z][a-z0-9_]*$`)
)

// inspections are the calls that read an error's kind or identity.
var inspections = []struct{ pkg, name string }{
	{where.Errx, "IsKind"},
	{"errors", "Is"},
	{"errors", "As"},
	{"errors", "AsType"},
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	switch {
	case where.Within(pkg.Rel, "internal/workers"):
		for _, file := range where.Files(pass) {
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && isHandle(pass, fn) {
					checkHandle(pass, fn)
				}
			}
		}
	case pkg.Rel == "internal/jobs":
		checkKinds(pass)
	}

	return nil, nil
}

// isHandle says whether fn is a job handler's Handle(ctx, job) error method.
func isHandle(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	if fn.Recv == nil || fn.Name.Name != "Handle" || fn.Body == nil {
		return false
	}

	obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}

	sig := obj.Signature()

	return sig.Params().Len() == 2 && where.IsContext(sig.Params().At(0).Type()) &&
		sig.Results().Len() == 1 && where.IsError(sig.Results().At(0).Type())
}

func checkHandle(pass *analysis.Pass, fn *ast.FuncDecl) {
	recv := pass.TypesInfo.Defs[fn.Name].(*types.Func).Signature().Recv()
	name := where.TypeName(recv.Type())

	if vagueNames[name] {
		pass.Reportf(fn.Name.Pos(), "a job handler is named for its action, such as Initiate or BuildCSV, not %s (JOB-1)", name)
	}

	ast.Inspect(fn.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		for _, inspection := range inspections {
			if where.Calls(pass, call, inspection.pkg, inspection.name) {
				pass.Reportf(call.Pos(), "a job handler returns its error and never inspects it: the runtime's one mapping picks the outcome (JOB-1, ERR-8)")
			}
		}

		return true
	})
}

// checkKinds reports a job kind constant whose value isn't capability.action.
func checkKinds(pass *analysis.Pass) {
	kind, _ := pass.Pkg.Scope().Lookup("Kind").(*types.TypeName)
	if kind == nil {
		return
	}

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}

			for _, spec := range gen.Specs {
				vs, _ := spec.(*ast.ValueSpec)
				for _, name := range vs.Names {
					obj, ok := pass.TypesInfo.Defs[name].(*types.Const)
					if !ok || !types.Identical(obj.Type(), kind.Type()) || obj.Val().Kind() != constant.String {
						continue
					}

					if value := constant.StringVal(obj.Val()); !kindValue.MatchString(value) {
						pass.Reportf(name.Pos(), "a job kind is <capability>.<action>, such as \"transfer.initiate\": %q (JOB-1)", value)
					}
				}
			}
		}
	}
}

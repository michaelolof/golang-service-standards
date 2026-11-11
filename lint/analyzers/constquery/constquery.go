// Package constquery checks the statements a service sends to its database.
//
//   - In a repository, the statement passed to a pgx, database/sql or gocql
//     query method is a constant expression: constants joined with +, never
//     built with Sprintf or a WHERE builder (DATA-5, SEC-15). A migration runner
//     or a tool may run statements it reads from files. gosec's G201 and G202
//     stay on everywhere as a backstop for string formatting.
//   - The statement doesn't read the database's clock: no now(),
//     current_timestamp or current_date (DATA-6). Nor does a MongoDB update:
//     no "$currentDate".
//   - ON CONFLICT names its target: ON CONFLICT (org_id, key) DO NOTHING
//     (DATA-8).
package constquery

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer checks database statements.
var Analyzer = &analysis.Analyzer{
	Name:     "constquery",
	Doc:      "checks that database statements are constants, read no database clock and name their conflict target",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

// drivers are the packages whose query methods take a statement.
var drivers = []string{
	"github.com/jackc/pgx", "database/sql", "github.com/gocql/gocql",
	"github.com/apache/cassandra-gocql-driver",
}

var methods = map[string]bool{
	"Query": true, "QueryRow": true, "Exec": true, "Queue": true, "Prepare": true,
	"QueryContext": true, "QueryRowContext": true, "ExecContext": true, "PrepareContext": true,
}

var (
	databaseClock     = regexp.MustCompile(`(?i)\bnow\s*\(\s*\)|\bcurrent_timestamp\b|\bcurrent_date\b`)
	conflictNoTarget  = regexp.MustCompile(`(?i)\bon\s+conflict\s+do\b`)
	mongoDatabaseTime = "$currentDate"
)

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if ok && where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	repository := ok && where.Within(pkg.Rel, "internal/repository")

	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil), (*ast.BasicLit)(nil)}, func(n ast.Node) {
		switch node := n.(type) {
		case *ast.CallExpr:
			checkCall(pass, node, repository)
		case *ast.BasicLit:
			if node.Kind == token.STRING {
				if s, err := strconv.Unquote(node.Value); err == nil && s == mongoDatabaseTime {
					pass.Reportf(node.Pos(), "time comes from the clock: set the field to now from clock.Clock, not $currentDate (DATA-6)")
				}
			}
		}
	})

	return nil, nil
}

func checkCall(pass *analysis.Pass, call *ast.CallExpr, repository bool) {
	fn := where.Callee(pass, call)
	if fn == nil || !methods[fn.Name()] || fn.Signature().Recv() == nil || !fromDriver(fn) {
		return
	}

	i := statementIndex(fn.Signature())
	if i < 0 || i >= len(call.Args) {
		return
	}

	arg := call.Args[i]

	value := pass.TypesInfo.Types[arg].Value
	if value == nil || value.Kind() != constant.String {
		if !repository {
			return
		}

		pass.Reportf(arg.Pos(), "a statement is a constant, declared in the method that runs it: join constants with +, and write optional filters as (@x::T IS NULL OR col = @x) (DATA-5)")
		return
	}

	statement := constant.StringVal(value)

	if databaseClock.MatchString(statement) {
		pass.Reportf(arg.Pos(), "time comes from the clock: pass now as a parameter instead of %s (DATA-6)", strings.ToLower(databaseClock.FindString(statement)))
	}

	if conflictNoTarget.MatchString(statement) {
		pass.Reportf(arg.Pos(), "name the conflict target, ON CONFLICT (cols) DO …, so another unique constraint's violation isn't swallowed (DATA-8)")
	}
}

func fromDriver(fn *types.Func) bool {
	if fn.Pkg() == nil {
		return false
	}

	path := fn.Pkg().Path()

	for _, driver := range drivers {
		if path == driver || strings.HasPrefix(path, driver+"/") {
			return true
		}
	}

	return false
}

// statementIndex returns the index of the first string parameter, which is the
// statement in every driver's query methods.
func statementIndex(sig *types.Signature) int {
	for i := range sig.Params().Len() {
		if basic, ok := sig.Params().At(i).Type().(*types.Basic); ok && basic.Kind() == types.String {
			return i
		}
	}

	return -1
}

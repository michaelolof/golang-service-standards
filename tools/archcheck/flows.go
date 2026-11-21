package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

const (
	flowsDir   = "test/flows"
	harnessDir = "internal/testutils/testharness"
	// bootstrapFile is the one place an identity no API can create, such as
	// the first admin, is seeded (TEST-16).
	bootstrapFile = harnessDir + "/bootstrap.go"
)

// checkFlowFiles reports a function or method declared in a file under
// test/flows, other than Test… and TestMain (TEST-19). A flow file holds
// tests. Steps, journeys, readers and request bodies live in the harness, where
// the next flow finds them. Package-level values, and types that hold a table's
// rows, are fine.
func checkFlowFiles(m *module) ([]problem, error) {
	var problems []problem

	visit := func(fset *token.FileSet, file *ast.File, rel string) {
		if !within(rel, flowsDir) {
			return
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || (fn.Recv == nil && (fn.Name.Name == "TestMain" || isTestFunc(fn.Name.Name, "Test"))) {
				continue
			}

			problems = append(problems, m.at(fset, fn.Name.Pos(), "%s is a helper in a flow file: move it into the harness, unchanged, and call it from there (TEST-19)", funcName(fn)))
		}
	}

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) { visit(p.Fset, file, rel) })

	for _, f := range m.ignoredFiles() {
		visit(f.fset, f.file, f.rel)
	}

	return problems, nil
}

// readPrefixes start the names of the repository methods that only read.
var readPrefixes = []string{"Get", "List", "Find", "Search", "Count", "Read", "Load"}

// drivers are the database drivers. A flow calls none of them, and the harness
// calls them only to read.
var drivers = []string{
	"database/sql", "github.com/jackc/pgx", "github.com/lib/pq", "github.com/jmoiron/sqlx",
	"github.com/gocql/gocql", "github.com/scylladb/gocqlx", "go.mongodb.org/mongo-driver",
}

// documentWrites are the methods of a document driver's collection that write,
// which have no statement to read.
var documentWrites = regexp.MustCompile(`^(Insert|Update|Replace|Delete|BulkWrite|FindOneAnd)`)

// boundaries is what every finding of checkFlowSetup asks for instead. The
// harness is also told where the one seed goes.
const (
	boundaries        = "set up through the API, the queued jobs and the vendor mock's webhooks (TEST-16)"
	harnessBoundaries = "set up through the API, the queued jobs and the vendor mock's webhooks, and seed only an identity no API can create, in " + bootstrapFile + " (TEST-16)"
)

// checkFlowSetup reports anything in test/flows, or in the harness, that sets
// up state some way a caller can't (TEST-16):
//   - an import of a repository's seed package, <aggregate>test under
//     internal/repository;
//   - a call to a repository method that writes: one that takes a context and
//     whose name doesn't start with a read, such as Get or List;
//   - in a flow, a call into a database driver, even to read, once per line: a
//     storage promise reads through a harness reader;
//   - in the harness, a statement that writes rows, such as INSERT or UPDATE;
//   - in a flow, an import of a fake beside an adapter or a port, which stands
//     in for the vendor mock.
//
// The harness's bootstrap.go may seed an identity no API can create. A harness
// method that passes the app's own call on to the repository, under the same
// name, such as one that keeps a test's keys apart, isn't a seed: the app
// makes that write. The job runner records each job's end through the queue
// adapter, as the queue does, so it isn't a seed either.
//
// Every flow is checked, an existing one too (TEST-14).
func checkFlowSetup(m *module) ([]problem, error) {
	var problems []problem

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) {
		inFlows := within(rel, flowsDir)
		if (!inFlows && !within(rel, harnessDir)) || rel == bootstrapFile {
			return
		}

		instead := harnessBoundaries
		if inFlows {
			instead = boundaries
		}

		for _, imp := range file.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)

			switch target, ok := m.relPkg(path); {
			case !ok:
			case within(target, "internal/repository") && isTestPackage(target, pkgName(p, path)):
				problems = append(problems, m.at(p.Fset, imp.Pos(), "imports the seed package %s: %s", path, instead))
			case inFlows && (within(target, "internal/adapters") || within(target, "internal/ports")) && isTestPackage(target, pkgName(p, path)):
				problems = append(problems, m.at(p.Fset, imp.Pos(), "imports %s, which stands in for the vendor mock: run against the mock, and %s", path, boundaries))
			}
		}

		// A flow's driver calls, one per line: a chain such as
		// pool.QueryRow(...).Scan(...) is one trip to the database.
		trips := map[int]driverCall{}

		for _, decl := range file.Decls {
			ast.Inspect(decl, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				fn, ok := typeutil.Callee(p.TypesInfo, call).(*types.Func)
				if !ok || fn.Pkg() == nil {
					return true
				}

				switch {
				case m.writesThroughRepository(fn) && !passesOn(decl, fn):
					problems = append(problems, m.at(p.Fset, call.Pos(), "%s writes through the repository: %s", calleeName(fn), instead))
				case inFlows && isDriver(fn.Pkg().Path()):
					line := p.Fset.Position(call.Pos()).Line
					if prev, seen := trips[line]; !seen || (!takesContext(prev.fn) && takesContext(fn)) {
						trips[line] = driverCall{call: call, fn: fn}
					}
				case isDriver(fn.Pkg().Path()) && writesRows(p.TypesInfo, fn, call):
					problems = append(problems, m.at(p.Fset, call.Pos(), "%s writes rows: the harness runs no SQL that writes, so %s", calleeName(fn), instead))
				}

				return true
			})
		}

		for _, trip := range trips {
			problems = append(problems, m.at(p.Fset, trip.call.Pos(), "%s reaches the database: a flow runs no SQL, even to read. Read storage through a harness reader, and %s", calleeName(trip.fn), boundaries))
		}
	})

	return problems, nil
}

// driverCall is one call into a database driver.
type driverCall struct {
	call *ast.CallExpr
	fn   *types.Func
}

// passesOn says whether decl is a method that passes a call of its own name on
// to fn, such as a wrapper the app calls that keeps a test's keys apart.
func passesOn(decl ast.Decl, fn *types.Func) bool {
	method, ok := decl.(*ast.FuncDecl)

	return ok && method.Recv != nil && method.Name.Name == fn.Name()
}

// takesContext says whether fn's first parameter is a context: the call that
// goes to the database, in a chain of driver calls.
func takesContext(fn *types.Func) bool {
	params := fn.Signature().Params()

	return params.Len() > 0 && isContext(params.At(0).Type())
}

// writesThroughRepository says whether fn is a repository method that writes:
// a method of a type in internal/repository, outside its seed packages, that
// takes a context and whose name doesn't start with a read. An accessor, such
// as a set's Orders(), takes no context.
func (m *module) writesThroughRepository(fn *types.Func) bool {
	sig := fn.Signature()
	if sig.Recv() == nil || sig.Params().Len() == 0 || !isContext(sig.Params().At(0).Type()) {
		return false
	}

	if rel, ok := m.relPkg(fn.Pkg().Path()); !ok || !within(rel, "internal/repository") || isTestPackage(rel, fn.Pkg().Name()) {
		return false
	}

	for _, prefix := range readPrefixes {
		if strings.HasPrefix(fn.Name(), prefix) {
			return false
		}
	}

	return true
}

// writesRows says whether a call into a database driver writes rows: a
// document driver's write method, or a statement whose text starts with a
// write. A statement built at run time, or one that changes the schema, such
// as the harness creating a package's database, isn't one.
func writesRows(info *types.Info, fn *types.Func, call *ast.CallExpr) bool {
	if strings.HasPrefix(fn.Pkg().Path(), "go.mongodb.org/") {
		recv := fn.Signature().Recv()

		return recv != nil && namedOf(recv.Type()) != nil && namedOf(recv.Type()).Obj().Name() == "Collection" && documentWrites.MatchString(fn.Name())
	}

	sig := fn.Signature()

	// The statement is the first string the driver takes. The values bound to
	// it come after, and may hold any word.
	for i, arg := range call.Args {
		if isString(paramType(sig, i)) {
			return isWriteStatement(leadingText(info, arg))
		}
	}

	return false
}

// paramType returns the type of the parameter the call's argument i fills.
func paramType(sig *types.Signature, i int) types.Type {
	params := sig.Params()

	switch {
	case sig.Variadic() && i >= params.Len()-1:
		if slice, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
			return slice.Elem()
		}
	case i < params.Len():
		return params.At(i).Type()
	}

	return nil
}

func isString(t types.Type) bool {
	basic, ok := types.Unalias(t).(*types.Basic)

	return ok && basic.Kind() == types.String
}

var (
	// sqlNoise is what can hold any word without being a statement: comments
	// and string literals.
	sqlNoise      = regexp.MustCompile(`(?s)--[^\n]*|/\*.*?\*/|'(?:[^']|'')*'`)
	writeKeywords = map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true, "UPSERT": true, "MERGE": true, "TRUNCATE": true, "REPLACE": true}
	// writeClause is a write inside a WITH or a CQL batch. SELECT … FOR UPDATE
	// has no SET, so it isn't one.
	writeClause = regexp.MustCompile(`(?i)\b(?:INSERT\s+INTO|UPSERT\s+INTO|MERGE\s+INTO|DELETE\s+FROM|UPDATE\s+[\w."]+(?:\s+(?:AS\s+)?\w+)?\s+SET)\b`)
)

// isWriteStatement says whether a statement's text writes rows: its first
// word is INSERT, UPDATE, DELETE, UPSERT, MERGE, TRUNCATE or REPLACE, or it's a
// WITH or a CQL batch that holds a write. Comments and string literals don't
// count.
func isWriteStatement(text string) bool {
	words := strings.Fields(sqlNoise.ReplaceAllString(text, " "))
	if len(words) == 0 {
		return false
	}

	switch first := strings.ToUpper(words[0]); {
	case writeKeywords[first]:
		return true
	case first == "WITH" || first == "BEGIN":
		return writeClause.MatchString(strings.Join(words, " "))
	default:
		return false
	}
}

// leadingText returns the text a string expression starts with, when the
// compiler knows it: a constant, the constant left side of a concatenation, or
// a constant format.
func leadingText(info *types.Info, expr ast.Expr) string {
	if tv, ok := info.Types[expr]; ok && tv.Value != nil && tv.Value.Kind() == constant.String {
		return constant.StringVal(tv.Value)
	}

	switch e := ast.Unparen(expr).(type) {
	case *ast.BinaryExpr:
		return leadingText(info, e.X)
	case *ast.CallExpr:
		if fn, ok := typeutil.Callee(info, e).(*types.Func); ok && fn.Pkg() != nil && fn.Pkg().Path() == "fmt" && fn.Name() == "Sprintf" && len(e.Args) > 0 {
			return leadingText(info, e.Args[0])
		}
	}

	return ""
}

// eachFile calls visit once for each Go file of the module, with its path
// relative to the root. A file in several builds of a package, such as a test
// build, is visited once.
func (m *module) eachFile(visit func(p *packages.Package, file *ast.File, rel string)) {
	seen := map[string]bool{}

	for _, p := range m.all {
		for _, file := range p.Syntax {
			path := p.Fset.Position(file.Package).Filename
			if seen[path] {
				continue
			}

			seen[path] = true
			visit(p, file, m.rel(path))
		}
	}
}

// isDriver says whether path is a database driver's package. Its encoding and
// option packages, such as bson, build values and reach nothing.
func isDriver(path string) bool {
	if strings.Contains(path, "/bson") || strings.HasSuffix(path, "/options") {
		return false
	}

	for _, d := range drivers {
		if path == d || strings.HasPrefix(path, d+"/") {
			return true
		}
	}

	return false
}

func isContext(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)

	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == "context" && named.Obj().Name() == "Context"
}

// isTestFunc says whether name is one go test runs with prefix, such as
// TestPlace or Test, the way the go tool reads it: the prefix, then nothing or
// a letter that isn't lower case.
func isTestFunc(name, prefix string) bool {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return false
	}

	if rest == "" {
		return true
	}

	r, _ := utf8.DecodeRuneInString(rest)

	return !unicode.IsLower(r)
}

// pkgName returns the name of the package p imports at path.
func pkgName(p *packages.Package, path string) string {
	if imp, ok := p.Imports[path]; ok && imp.Types != nil {
		return imp.Types.Name()
	}

	return path[strings.LastIndex(path, "/")+1:]
}

// funcName names a declared function the way a reader looks for it: Place, or
// World.Place for a method.
func funcName(fn *ast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	recv := fn.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}

	switch r := recv.(type) {
	case *ast.IndexExpr:
		recv = r.X
	case *ast.IndexListExpr:
		recv = r.X
	}

	if id, ok := recv.(*ast.Ident); ok {
		return id.Name + "." + fn.Name.Name
	}

	return fn.Name.Name
}

// calleeName names a called function with its package or receiver, such as
// pgxpool.Pool.Exec or orders.Repo.MarkPaid.
func calleeName(fn *types.Func) string {
	if recv := fn.Signature().Recv(); recv != nil {
		if named := namedOf(recv.Type()); named != nil {
			return fn.Pkg().Name() + "." + named.Obj().Name() + "." + fn.Name()
		}
	}

	return fn.Pkg().Name() + "." + fn.Name()
}

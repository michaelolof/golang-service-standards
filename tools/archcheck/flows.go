package main

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"path/filepath"
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

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) {
		if !within(rel, flowsDir) {
			return
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || (fn.Recv == nil && (fn.Name.Name == "TestMain" || isTestFunc(fn.Name.Name, "Test"))) {
				continue
			}

			problems = append(problems, m.at(p.Fset, fn.Name.Pos(), "%s is a helper in a flow file: move it into the harness, unchanged, and call it from there (TEST-19)", funcName(fn)))
		}
	})

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
// Under TEST-14's pause, an existing flow keeps its setup until the lift, and
// its area's promise file lists it under "Not yet proved as the caller". Those
// flows, and the code only they reach, aren't reported. Code any other test
// reaches is.
func checkFlowSetup(m *module) ([]problem, error) {
	paused, err := m.pausedFlows()
	if err != nil {
		return nil, err
	}

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
			case !ok, paused.acceptsImport(p, file, path):
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
			if paused.accepts(p.Fset, decl) {
				continue
			}

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

// pausedFlows is what the flows under "Not yet proved as the caller" keep until
// the engineer lifts TEST-14: the functions of test code no other test reaches.
type pausedFlows struct {
	listed bool
	// reached holds each function of test code some other test reaches, by the
	// position of its name.
	reached map[token.Position]bool
}

// pausedFlows reads the tests the promise files list under "Not yet proved as
// the caller", and follows every reference to a function of test code from
// every other test. A function only the listed flows reach is theirs.
func (m *module) pausedFlows() (pausedFlows, error) {
	listed, err := m.notYetProved()
	if err != nil || len(listed) == 0 {
		return pausedFlows{}, err
	}

	refs := map[token.Position][]token.Position{}

	var others []token.Position

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) {
		if !isTestCode(m, p, rel) {
			return
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}

			at := p.Fset.Position(fn.Name.Pos())
			if strings.HasSuffix(rel, "_test.go") && fn.Recv == nil && runByGoTest(fn.Name.Name) && !listed[fn.Name.Name] {
				others = append(others, at)
			}

			ast.Inspect(fn, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					if used, ok := p.TypesInfo.Uses[id].(*types.Func); ok && used.Pos().IsValid() {
						refs[at] = append(refs[at], p.Fset.Position(used.Pos()))
					}
				}

				return true
			})
		}
	})

	reached := map[token.Position]bool{}

	for len(others) > 0 {
		at := others[len(others)-1]
		others = others[:len(others)-1]

		if !reached[at] {
			reached[at] = true
			others = append(others, refs[at]...)
		}
	}

	return pausedFlows{listed: true, reached: reached}, nil
}

// accepts says whether decl is a function only the listed flows reach.
func (pf pausedFlows) accepts(fset *token.FileSet, decl ast.Decl) bool {
	fn, ok := decl.(*ast.FuncDecl)

	return pf.listed && ok && !pf.reached[fset.Position(fn.Name.Pos())]
}

// acceptsImport says whether only functions the listed flows reach use the
// package file imports at path.
func (pf pausedFlows) acceptsImport(p *packages.Package, file *ast.File, path string) bool {
	if !pf.listed {
		return false
	}

	for _, decl := range file.Decls {
		if refersTo(p.TypesInfo, decl, path) && !pf.accepts(p.Fset, decl) {
			return false
		}
	}

	return true
}

// refersTo says whether node names something from the package at path.
func refersTo(info *types.Info, node ast.Node, path string) bool {
	found := false

	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if pkg, ok := info.Uses[id].(*types.PkgName); ok && pkg.Imported().Path() == path {
				found = true
			}
		}

		return !found
	})

	return found
}

// notYetProved returns the tests the promise files name under "Not yet proved
// as the caller" (TEST-14, TEST-18).
func (m *module) notYetProved() (map[string]bool, error) {
	paths, err := filepath.Glob(filepath.Join(m.root, promisesDir, "*.md"))
	if err != nil {
		return nil, err
	}

	listed := map[string]bool{}

	for _, path := range paths {
		f, err := readPromiseFile(m.rel(path), path)
		if err != nil {
			return nil, err
		}

		for _, e := range f.entries {
			if e.section != notYetProved {
				continue
			}

			for _, t := range testsIn(e.text) {
				listed[t] = true
			}
		}
	}

	return listed, nil
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

	for _, arg := range call.Args {
		if isWriteStatement(leadingText(info, arg)) {
			return true
		}
	}

	return false
}

var (
	sqlComments    = regexp.MustCompile(`(?s)^(\s*(--[^\n]*\n|/\*.*?\*/))*\s*`)
	writeKeywords  = map[string]bool{"INSERT": true, "UPDATE": true, "DELETE": true, "UPSERT": true, "MERGE": true, "TRUNCATE": true, "REPLACE": true}
	writeInsideCTE = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|UPSERT|MERGE)\b`)
)

// isWriteStatement says whether a statement's text writes rows: it starts with
// INSERT, UPDATE, DELETE, UPSERT, MERGE, TRUNCATE or REPLACE, or it's a WITH or
// a CQL batch that holds one of them.
func isWriteStatement(text string) bool {
	text = sqlComments.ReplaceAllString(text, "")
	first, _, _ := strings.Cut(text, " ")
	first = strings.ToUpper(strings.TrimSpace(first))

	switch {
	case writeKeywords[first]:
		return true
	case first == "WITH" || first == "BEGIN":
		return writeInsideCTE.MatchString(text)
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

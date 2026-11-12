// Package txshape checks the transactions core opens, and the ports that may
// join them (DATA-4).
//
// A transaction that core opens is a call to a method named Within that takes a
// func(context.Context) error, in a function or in the function literal of a
// package-level variable, such as a gofi handler. It holds exactly one
// business write plus boundary writes, and nothing else. A boundary port is an interface that
// embeds txboundary.Port, from internal/foundation/txboundary, itself or
// through an interface it embeds.
//
// In the closure, and in the functions of the same package it calls, each call
// that is given the ctx is one of these:
//   - a boundary write, through a boundary port;
//   - a read, through a Repo or an interface, when its verb is Get, Find, List
//     or Count, or a banned verb that reads (DATA-3). Reported: the guard goes
//     in the statement's WHERE clause;
//   - a call to a shared port, in ports, jobs or foundation, that isn't a
//     boundary port. Reported;
//   - a business write, through a Repo or any other interface. The second one
//     is reported, and so is one inside a loop;
//   - anything else in the module that txshape can't see into: a function or
//     method of another package, or a func value. Reported, because it can
//     hide a write. Functions in foundation, and calls that only read or derive
//     the ctx (CTX-2), are left alone.
//
// A closure with no business write, or with no boundary write, is reported at
// the Within call, unless it already has a finding.
//
// Outside the closures:
//   - A function that takes a func(context.Context) error and runs it in Within
//     is a generic wrapper. The transaction adapter's own Within is exempt.
//   - A boundary port's method that takes a callback is reported, and so is a
//     call that opens a transaction inside a method that implements one.
//
// Repositories are only checked for the last of these. A transaction a
// repository opens inside its own method is DATA-4's database-limit exception,
// and has no fixed shape. A call to an external system through an interface
// core declares counts as a business write, because nothing tells it apart
// from a repository: that is for review.
package txshape

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/repo"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks transactions and boundary ports.
var Analyzer = &analysis.Analyzer{
	Name: "txshape",
	Doc:  "checks that a transaction core opens holds one business write and its boundary writes, and that boundary ports join it and never open one",
	Run:  run,
}

// marker is the folder, under the module, that declares txboundary.Port.
const marker = "internal/foundation/txboundary"

// reads are the verbs that read: DATA-3's, and the banned ones that also read.
var reads = map[string]bool{
	"Get": true, "Find": true, "List": true, "Count": true,
	"Fetch": true, "Lookup": true, "Load": true, "Read": true, "Search": true,
}

// openers are the calls that open a transaction, in the drivers and in the
// store adapters.
var openers = map[string]bool{
	"Begin": true, "BeginTx": true, "BeginFunc": true, "BeginTxFunc": true,
	"StartSession": true, "WithTransaction": true, "WithinTx": true, "Within": true,
}

type checker struct {
	pass  *analysis.Pass
	pkg   where.Package
	decls map[*types.Func]*ast.FuncDecl
	// seen holds what's been reported, so a function followed from two
	// closures isn't reported twice for the same thing.
	seen map[string]bool
}

// tx is one transaction's closure, as it's walked.
type tx struct {
	at token.Pos
	// callbacks are the enclosing function's func(context.Context) error
	// parameters. checkWrappers reports them, so calling one isn't a finding.
	callbacks  map[types.Object]bool
	visited    map[*ast.FuncDecl]bool
	writes     int
	boundaries int
	wrapped    int
	findings   int
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.IsTestInfra(pkg.Rel, pkg.Name) || where.Within(pkg.Rel, "pkg") || where.Within(pkg.Rel, marker) {
		return nil, nil
	}

	c := &checker{pass: pass, pkg: pkg, decls: map[*types.Func]*ast.FuncDecl{}, seen: map[string]bool{}}

	for _, fn := range c.funcDecls() {
		if obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func); ok {
			c.decls[obj] = fn
		}
	}

	c.checkPorts()
	c.checkImplementations()

	if !where.Within(pkg.Rel, "internal/repository") {
		c.checkTransactions()
		c.checkWrappers()
	}

	return nil, nil
}

// checkTransactions checks each closure passed to Within, in functions and in
// the function literals of package-level variables, such as gofi's handlers
// (HTTP-1). A Within inside another's closure joins that transaction, so it's
// walked as part of it.
func (c *checker) checkTransactions() {
	for _, file := range where.Files(c.pass) {
		var stack []ast.Node

		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]

				return true
			}

			if call, ok := n.(*ast.CallExpr); ok && c.isWithin(call) {
				c.checkTransaction(call, stack)

				return false
			}

			stack = append(stack, n)

			return true
		})
	}
}

// checkTransaction walks the closure call passes to Within. stack holds the
// nodes around call, so the function it's in can be found.
func (c *checker) checkTransaction(call *ast.CallExpr, stack []ast.Node) {
	body := c.closure(call.Args[1])
	if body == nil {
		return
	}

	t := &tx{at: call.Pos(), callbacks: map[types.Object]bool{}, visited: map[*ast.FuncDecl]bool{}}

	for i := len(stack) - 1; i >= 0; i-- {
		if decl, ok := stack[i].(*ast.FuncDecl); ok {
			t.callbacks, t.visited[decl] = c.callbacks(decl.Type), true

			break
		}

		if lit, ok := stack[i].(*ast.FuncLit); ok {
			t.callbacks = c.callbacks(lit.Type)

			break
		}
	}

	c.walk(t, body, false, false)
	c.summarize(t)
}

// summarize reports a transaction that holds no business write, or no boundary
// write. A closure that already has a finding, or runs a wrapper's callback,
// isn't summarized: its shape is already wrong.
func (c *checker) summarize(t *tx) {
	if t.findings > 0 || t.wrapped > 0 {
		return
	}

	switch {
	case t.writes == 0:
		c.report(t.at, "the transaction makes no business write: a transaction that core opens holds one use-case write and its boundary writes (DATA-4)")
	case t.boundaries == 0:
		c.report(t.at, "the transaction holds one business write and no boundary write, so it has no reason to exist: call the use-case method on its own (DATA-4)")
	}
}

// walk classifies each call in body. inLoop says whether body runs once per
// loop iteration, and followed whether body is a function the closure calls.
func (c *checker) walk(t *tx, body ast.Node, inLoop, followed bool) {
	var stack []ast.Node

	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]

			return true
		}

		stack = append(stack, n)

		if call, ok := n.(*ast.CallExpr); ok {
			c.classify(t, call, inLoop || inLoopBody(stack, call), followed)
		}

		return true
	})
}

// inLoopBody says whether call is inside the body of a for or range statement
// on stack.
func inLoopBody(stack []ast.Node, call *ast.CallExpr) bool {
	for _, n := range stack {
		var body *ast.BlockStmt

		switch loop := n.(type) {
		case *ast.ForStmt:
			body = loop.Body
		case *ast.RangeStmt:
			body = loop.Body
		}

		if body != nil && body.Pos() <= call.Pos() && call.End() <= body.End() {
			return true
		}
	}

	return false
}

func (c *checker) classify(t *tx, call *ast.CallExpr, inLoop, followed bool) {
	if c.isWithin(call) {
		// It joins this transaction. A func literal's body is walked as part of
		// this closure; a function of this package is followed.
		if decl := c.declOf(call.Args[1]); decl != nil {
			c.follow(t, decl, inLoop)
		}

		return
	}

	if !c.givenCtx(call) {
		return
	}

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		selection := c.pass.TypesInfo.Selections[sel]
		if selection != nil && selection.Kind() == types.MethodVal && c.isDependency(selection.Recv()) {
			c.dependency(t, call, c.describe(selection.Recv(), sel.Sel.Name), selection.Recv(), sel.Sel.Name, inLoop, followed)

			return
		}
	}

	callee := where.Callee(c.pass, call)
	if callee == nil {
		if id, ok := ast.Unparen(call.Fun).(*ast.Ident); ok && t.callbacks[c.pass.TypesInfo.Uses[id]] {
			t.wrapped++

			return
		}

		if _, ok := c.pass.TypesInfo.TypeOf(call.Fun).Underlying().(*types.Signature); ok {
			c.find(t, call.Pos(), followed, "a func value is given the transaction's ctx, and txshape can't see what it writes: call the repository method or boundary port directly in the closure (DATA-4)")
		}

		return
	}

	if decl := c.decls[callee.Origin()]; decl != nil {
		c.follow(t, decl, inLoop)

		return
	}

	if callee.Pkg() == nil || isCtxAccessor(callee) {
		return
	}

	rel, inModule := c.pkg.Import(callee.Pkg().Path())
	if !inModule || where.Within(rel, "internal/foundation") {
		return
	}

	c.find(t, call.Pos(), followed, "%s is given the transaction's ctx, and txshape can't see what it writes: call the repository method or boundary port directly in the closure (DATA-4)", c.funcName(callee))
}

// dependency classifies a call through an interface or a Repo.
func (c *checker) dependency(t *tx, call *ast.CallExpr, name string, recv types.Type, method string, inLoop, followed bool) {
	switch {
	case c.isBoundary(recv):
		t.boundaries++
	case reads[repo.Verb(method)]:
		c.find(t, call.Pos(), followed, "%s reads inside the transaction: put the guard in the statement's WHERE clause instead, so nothing can change between the check and the write (DATA-4, DATA-7)", name)
	case c.isShared(recv):
		c.find(t, call.Pos(), followed, "%s isn't a boundary port: its write goes in the use-case statement, or after the commit. Only a port that passes DATA-4's three tests embeds txboundary.Port (DATA-4)", name)
	default:
		t.writes++

		switch {
		case t.writes > 1:
			c.find(t, call.Pos(), followed, "%s is a second business write in the transaction: one use-case repository method makes the whole change, in one statement, or in a transaction it opens itself where the database can't (DATA-4)", name)
		case inLoop:
			c.find(t, call.Pos(), followed, "%s runs in a loop, so the transaction makes several business writes: one use-case repository method writes every row, in one statement, or in a transaction it opens itself where the database can't (DATA-4)", name)
		}
	}
}

func (c *checker) follow(t *tx, decl *ast.FuncDecl, inLoop bool) {
	if t.visited[decl] {
		return
	}

	t.visited[decl] = true
	c.walk(t, decl.Body, inLoop, true)
}

// checkWrappers reports a function, or a function literal, that takes a
// func(context.Context) error and runs it in a transaction. The transaction
// adapter's own Within is exempt: it's what runs the callback.
func (c *checker) checkWrappers() {
	const wrapper = "takes a func(context.Context) error and runs it in a transaction: a generic wrapper lets any number of writes share one. Open the transaction in the use case, around its one write and its boundary writes (DATA-4)"

	for _, file := range where.Files(c.pass) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch fn := n.(type) {
			case *ast.FuncDecl:
				if fn.Body != nil && (fn.Recv == nil || fn.Name.Name != "Within") && c.passesTo(fn.Body, c.callbacks(fn.Type)) {
					c.report(fn.Name.Pos(), fn.Name.Name+" "+wrapper)
				}
			case *ast.FuncLit:
				if c.passesTo(fn.Body, c.callbacks(fn.Type)) {
					c.report(fn.Pos(), "this function "+wrapper)
				}
			}

			return true
		})
	}
}

// passesTo says whether body passes one of callbacks to Within, directly or
// inside the closure it gives it.
func (c *checker) passesTo(body *ast.BlockStmt, callbacks map[types.Object]bool) bool {
	if len(callbacks) == 0 {
		return false
	}

	found := false

	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if found || !ok || !c.isWithin(call) {
			return !found
		}

		for _, arg := range call.Args {
			ast.Inspect(arg, func(m ast.Node) bool {
				if id, ok := m.(*ast.Ident); ok && callbacks[c.pass.TypesInfo.Uses[id]] {
					found = true
				}

				return !found
			})
		}

		return !found
	})

	return found
}

// checkPorts reports a boundary port's method that takes a callback. A
// boundary port writes its own record in the caller's transaction, so it never
// runs the caller's code.
func (c *checker) checkPorts() {
	for _, file := range where.Files(c.pass) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}

			for _, spec := range gen.Specs {
				obj, _ := c.pass.TypesInfo.Defs[spec.(*ast.TypeSpec).Name].(*types.TypeName)
				if obj == nil || !c.isBoundary(obj.Type()) {
					continue
				}

				iface, _ := obj.Type().Underlying().(*types.Interface)
				for method := range iface.ExplicitMethods() {
					if takesFunc(method.Signature()) {
						c.report(method.Pos(), fmt.Sprintf("%s.%s takes a callback: a boundary port writes its own record in the caller's transaction, and never runs the caller's code (DATA-4)", obj.Name(), method.Name()))
					}
				}
			}
		}
	}
}

// checkImplementations reports a call that opens a transaction inside a method
// that implements a boundary port. It finds the implementations by their
// assertion, var _ Port = (*T)(nil), which ABS-2 requires.
func (c *checker) checkImplementations() {
	for _, file := range where.Files(c.pass) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}

			for _, spec := range gen.Specs {
				vs, _ := spec.(*ast.ValueSpec)
				if vs.Type == nil || len(vs.Values) != len(vs.Names) {
					continue
				}

				port := c.pass.TypesInfo.TypeOf(vs.Type)
				if port == nil || !c.isBoundary(port) {
					continue
				}

				for i, name := range vs.Names {
					if name.Name == "_" {
						c.checkImplementation(c.pass.TypesInfo.TypeOf(vs.Values[i]), port)
					}
				}
			}
		}
	}
}

func (c *checker) checkImplementation(impl, port types.Type) {
	iface, _ := port.Underlying().(*types.Interface)

	for method := range iface.Methods() {
		obj, _, _ := types.LookupFieldOrMethod(impl, true, c.pass.Pkg, method.Name())

		fn, _ := obj.(*types.Func)
		if fn == nil || c.decls[fn] == nil {
			continue
		}

		name := c.describe(impl, fn.Name())
		visited := map[*ast.FuncDecl]bool{}

		var walk func(decl *ast.FuncDecl)

		walk = func(decl *ast.FuncDecl) {
			if visited[decl] {
				return
			}

			visited[decl] = true

			ast.Inspect(decl.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}

				callee := where.Callee(c.pass, call)
				if callee == nil {
					return true
				}

				if openers[callee.Name()] {
					c.report(call.Pos(), fmt.Sprintf("%s implements the boundary port %s, so it joins the caller's transaction and never opens one: %s opens a transaction (DATA-4)", name, c.describe(port, ""), callee.Name()))
				} else if inner := c.decls[callee.Origin()]; inner != nil {
					walk(inner)
				}

				return true
			})
		}

		walk(c.decls[fn])
	}
}

// isWithin says whether call opens a transaction: a method named Within that
// takes a ctx and a func(context.Context) error.
func (c *checker) isWithin(call *ast.CallExpr) bool {
	callee := where.Callee(c.pass, call)
	if callee == nil || callee.Name() != "Within" {
		return false
	}

	sig := callee.Signature()

	return sig.Recv() != nil && sig.Params().Len() == 2 && len(call.Args) == 2 &&
		where.IsContext(sig.Params().At(0).Type()) && isCallback(sig.Params().At(1).Type())
}

// closure returns the body Within runs: a func literal's, or the declaration
// of a function or method of this package. It returns nil for anything else,
// such as a parameter, which checkWrappers reports.
func (c *checker) closure(arg ast.Expr) ast.Node {
	if lit, ok := ast.Unparen(arg).(*ast.FuncLit); ok {
		return lit.Body
	}

	if decl := c.declOf(arg); decl != nil {
		return decl.Body
	}

	return nil
}

// declOf returns the declaration of the function or method of this package
// that expr names, or nil.
func (c *checker) declOf(expr ast.Expr) *ast.FuncDecl {
	var id *ast.Ident

	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return nil
	}

	fn, _ := c.pass.TypesInfo.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}

	return c.decls[fn.Origin()]
}

// callbacks returns a function's parameters of type func(context.Context) error.
func (c *checker) callbacks(fn *ast.FuncType) map[types.Object]bool {
	found := map[types.Object]bool{}

	for _, field := range fn.Params.List {
		for _, name := range field.Names {
			if obj := c.pass.TypesInfo.Defs[name]; obj != nil && isCallback(obj.Type()) {
				found[obj] = true
			}
		}
	}

	return found
}

// givenCtx says whether one of call's arguments is a context.Context.
func (c *checker) givenCtx(call *ast.CallExpr) bool {
	for _, arg := range call.Args {
		if where.IsContext(c.pass.TypesInfo.TypeOf(arg)) {
			return true
		}
	}

	return false
}

// isDependency says whether a method call on t goes to a dependency: through
// an interface, or to a repository's Repo.
func (c *checker) isDependency(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Interface); ok {
		return true
	}

	return repo.IsRepo(t, c.pkg.Module)
}

// isBoundary says whether t is an interface that embeds txboundary.Port,
// itself or through an interface it embeds.
func (c *checker) isBoundary(t types.Type) bool {
	return c.embedsMarker(t, map[types.Type]bool{})
}

func (c *checker) embedsMarker(t types.Type, seen map[types.Type]bool) bool {
	t = types.Unalias(t)
	if seen[t] {
		return false
	}

	seen[t] = true

	iface, ok := t.Underlying().(*types.Interface)
	if !ok {
		return false
	}

	for embedded := range iface.EmbeddedTypes() {
		if where.IsNamed(embedded, c.pkg.Module+"/"+marker, "Port") || c.embedsMarker(embedded, seen) {
			return true
		}
	}

	return false
}

// isShared says whether t is declared in a layer that shares ports between
// packages: ports, jobs or foundation.
func (c *checker) isShared(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}

	rel, ok := c.pkg.Import(named.Obj().Pkg().Path())
	if !ok {
		return false
	}

	layer, _ := where.Layer(rel)

	return layer == "ports" || layer == "jobs" || layer == "foundation"
}

// describe names a method of t as Type.Method, with the package's name when t
// is from another package. An empty method names the type alone.
func (c *checker) describe(t types.Type, method string) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	name := method

	if named, ok := types.Unalias(t).(*types.Named); ok {
		obj := named.Obj()

		name = obj.Name()
		if obj.Pkg() != nil && obj.Pkg() != c.pass.Pkg {
			name = obj.Pkg().Name() + "." + name
		}

		if method != "" {
			name += "." + method
		}
	}

	return name
}

func (c *checker) funcName(fn *types.Func) string {
	if recv := fn.Signature().Recv(); recv != nil {
		return c.describe(recv.Type(), fn.Name())
	}

	return fn.Pkg().Name() + "." + fn.Name()
}

// find reports a finding in t. One in a function the closure calls says which
// transaction it's in.
func (c *checker) find(t *tx, pos token.Pos, followed bool, format string, args ...any) {
	t.findings++

	msg := fmt.Sprintf(format, args...)
	if followed {
		at := c.pass.Fset.Position(t.at)
		msg = fmt.Sprintf("called in the transaction at %s:%d: %s", filepath.Base(at.Filename), at.Line, msg)
	}

	c.report(pos, msg)
}

func (c *checker) report(pos token.Pos, msg string) {
	key := fmt.Sprint(pos, msg)
	if c.seen[key] {
		return
	}

	c.seen[key] = true
	c.pass.Report(analysis.Diagnostic{Pos: pos, Message: msg})
}

// funcDecls returns the package's function declarations that have a body, in
// file order, without tests.
func (c *checker) funcDecls() []*ast.FuncDecl {
	var decls []*ast.FuncDecl

	for _, file := range where.Files(c.pass) {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Body != nil {
				decls = append(decls, fn)
			}
		}
	}

	return decls
}

// isCallback says whether t is func(context.Context) error.
func isCallback(t types.Type) bool {
	sig, ok := t.Underlying().(*types.Signature)

	return ok && !sig.Variadic() && sig.Params().Len() == 1 && where.IsContext(sig.Params().At(0).Type()) &&
		sig.Results().Len() == 1 && where.IsError(sig.Results().At(0).Type())
}

// isCtxAccessor says whether fn only reads or derives a ctx, so it writes
// nothing: it returns a ctx, as WithX and context.WithTimeout do, or it's an
// XFrom accessor that takes only the ctx (CTX-2).
func isCtxAccessor(fn *types.Func) bool {
	sig := fn.Signature()

	if sig.Results().Len() > 0 && where.IsContext(sig.Results().At(0).Type()) {
		return true
	}

	return strings.HasSuffix(fn.Name(), "From") && sig.Params().Len() == 1 && where.IsContext(sig.Params().At(0).Type())
}

// takesFunc says whether one of sig's parameters is a function.
func takesFunc(sig *types.Signature) bool {
	for param := range sig.Params().Variables() {
		if _, ok := param.Type().Underlying().(*types.Signature); ok {
			return true
		}
	}

	return false
}

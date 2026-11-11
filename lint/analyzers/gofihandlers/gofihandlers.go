// Package gofihandlers checks HTTP handlers against gofi's GUIDELINES.md
// (v0.10.4), which HTTP-1 and HTTP-2 adopt. It checks gofi's conventions, not
// the standard's, so it could move into gofi. For every package-level variable
// set to gofi.DefineHandler(...) under internal/http:
//
//   - it's exported and named <Operation>Handler (HND-1, HND-2);
//   - it's the only one in its file (LAY-2);
//   - its RouteOptions fields are in the order Info, Schema, Meta, Handler (HND-3);
//   - its schema is an unexported <operation>Schema, declared directly below it
//     in the same file (HND-4);
//   - its handler reads request data from the schema, not from c.Param, c.Query
//     or c.HeaderVal (HND-8);
//   - its Info has exactly one lowercase kebab-case tag, the package is named
//     after that tag, and it has a Description (LAY-3, DOC-1). A Hidden route
//     isn't in the OpenAPI document, so its Info isn't checked.
//
// The standard adds one rule to gofi's, from HTTP-5: no OperationId.
package gofihandlers

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"regexp"
	"strings"
	"unicode"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks gofi handler declarations.
var Analyzer = &analysis.Analyzer{
	Name: "gofihandlers",
	Doc:  "checks gofi handlers against gofi's GUIDELINES.md: naming, one per file, field order, schema placement and route info",
	Run:  run,
}

var kebab = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

// fieldOrder is HND-3's order of RouteOptions fields.
var fieldOrder = map[string]int{"Info": 0, "Schema": 1, "Meta": 2, "Handler": 3}

// requestAccessors are the gofi.Context methods a handler doesn't use (HND-8).
var requestAccessors = map[string]bool{"Param": true, "Query": true, "HeaderVal": true}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !where.Within(pkg.Rel, "internal/http") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	for _, file := range where.Files(pass) {
		handlers := 0

		for i, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}

			for _, spec := range gen.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
					continue
				}

				opts, ok := definition(pass, vs.Values[0])
				if !ok {
					continue
				}

				handlers++
				if handlers > 1 {
					pass.Reportf(vs.Names[0].Pos(), "one operation per file: move %s to a file named for it (HTTP-2, gofi LAY-2)", vs.Names[0].Name)
				}

				h := handler{pass: pass, name: vs.Names[0], opts: opts, pkgName: pkg.Name}
				h.check(nextDecl(file.Decls, i))
			}
		}
	}

	return nil, nil
}

// definition returns the RouteOptions literal of gofi.DefineHandler(gofi.RouteOptions{...}).
func definition(pass *analysis.Pass, expr ast.Expr) (*ast.CompositeLit, bool) {
	call, ok := ast.Unparen(expr).(*ast.CallExpr)
	if !ok || len(call.Args) != 1 || !where.Calls(pass, call, where.Gofi, "DefineHandler") {
		return nil, false
	}

	lit, ok := ast.Unparen(call.Args[0]).(*ast.CompositeLit)

	return lit, ok
}

func nextDecl(decls []ast.Decl, i int) ast.Decl {
	if i+1 < len(decls) {
		return decls[i+1]
	}

	return nil
}

type handler struct {
	pass    *analysis.Pass
	name    *ast.Ident
	opts    *ast.CompositeLit
	pkgName string
}

func (h handler) check(next ast.Decl) {
	name := h.name.Name
	operation := strings.TrimSuffix(name, "Handler")

	if !h.name.IsExported() || operation == name || operation == "" {
		h.report("a handler is an exported variable named <Operation>Handler, such as CancelTransferHandler: rename %s (gofi HND-1, HND-2)", name)
	}

	last := -1

	for _, elt := range h.opts.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, _ := kv.Key.(*ast.Ident)
		if key == nil {
			continue
		}

		if order, ok := fieldOrder[key.Name]; ok {
			if order < last {
				h.report("%s's fields go in the order Info, Schema, Meta, Handler (gofi HND-3)", name)
			}

			last = order
		}

		switch key.Name {
		case "Info":
			h.checkInfo(kv.Value)
		case "Schema":
			h.checkSchema(kv.Value, operation, next)
		case "Handler":
			h.checkRequestReads(kv.Value)
		}
	}
}

// checkSchema reports a schema that isn't an unexported <operation>Schema
// declared directly below the handler.
func (h handler) checkSchema(expr ast.Expr, operation string, next ast.Decl) {
	t := h.pass.TypesInfo.TypeOf(expr)
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() != h.pass.Pkg {
		return
	}

	want := lowerInitial(operation) + "Schema"
	if named.Obj().Name() != want {
		h.report("%s's schema is an unexported type named %s (gofi HND-4)", h.name.Name, want)
		return
	}

	gen, ok := next.(*ast.GenDecl)
	if !ok || gen.Tok != token.TYPE || len(gen.Specs) == 0 {
		h.report("%s is declared directly below %s, in the same file (gofi HND-4)", want, h.name.Name)
		return
	}

	if ts, ok := gen.Specs[0].(*ast.TypeSpec); !ok || h.pass.TypesInfo.Defs[ts.Name] != named.Obj() {
		h.report("%s is declared directly below %s, in the same file (gofi HND-4)", want, h.name.Name)
	}
}

// checkInfo reports Info without exactly one kebab-case tag naming the package,
// without a Description, or with an OperationId.
func (h handler) checkInfo(expr ast.Expr) {
	info, ok := ast.Unparen(expr).(*ast.CompositeLit)
	if !ok {
		return
	}

	fields := map[string]ast.Expr{}

	for _, elt := range info.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok {
				fields[key.Name] = kv.Value
			}
		}
	}

	if hidden, ok := fields["Hidden"]; ok && h.isTrue(hidden) {
		return // a hidden route isn't in the OpenAPI document
	}

	if _, ok := fields["OperationId"]; ok {
		h.report("%s sets OperationId: leave it out, the method and path identify the route (HTTP-5)", h.name.Name)
	}

	if !h.nonEmptyString(fields["Description"]) {
		h.report("%s's Info needs a Description written for API consumers (gofi DOC-1, DOC-2)", h.name.Name)
	}

	tags, ok := fields["Tags"]
	if !ok {
		h.report("%s's Info has exactly one tag, the one its package is named after (gofi DOC-1)", h.name.Name)
		return
	}

	h.checkTags(tags)
}

func (h handler) checkTags(expr ast.Expr) {
	lit, ok := ast.Unparen(expr).(*ast.CompositeLit)
	if !ok {
		return
	}

	if len(lit.Elts) != 1 {
		h.report("%s's Info has exactly one tag, the one its package is named after (gofi DOC-1)", h.name.Name)
		return
	}

	value := h.pass.TypesInfo.Types[lit.Elts[0]].Value
	if value == nil || value.Kind() != constant.String {
		return
	}

	tag := constant.StringVal(value)

	switch {
	case !kebab.MatchString(tag):
		h.report("the tag %q is lowercase kebab-case (gofi DOC-1)", tag)
	case strings.ReplaceAll(tag, "-", "") != h.pkgName:
		h.report("the package is named after its OpenAPI tag: tag %q goes in package %s, not %s (HTTP-2, gofi LAY-3)", tag, strings.ReplaceAll(tag, "-", ""), h.pkgName)
	}
}

// checkRequestReads reports c.Param, c.Query and c.HeaderVal inside the handler.
func (h handler) checkRequestReads(expr ast.Expr) {
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		fn := where.Callee(h.pass, call)
		if fn == nil || !requestAccessors[fn.Name()] || fn.Pkg() == nil || fn.Pkg().Path() != where.Gofi {
			return true
		}

		if recv := fn.Signature().Recv(); recv != nil {
			h.pass.Reportf(call.Pos(), "a handler reads request data from its bound schema, not from c.%s (gofi HND-8)", fn.Name())
		}

		return true
	})
}

func (h handler) isTrue(expr ast.Expr) bool {
	value := h.pass.TypesInfo.Types[expr].Value

	return value != nil && value.Kind() == constant.Bool && constant.BoolVal(value)
}

func (h handler) nonEmptyString(expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	value := h.pass.TypesInfo.Types[expr].Value
	if value == nil || value.Kind() != constant.String {
		return true // built elsewhere: review reads it
	}

	return strings.TrimSpace(constant.StringVal(value)) != ""
}

func (h handler) report(format string, args ...any) {
	h.pass.Reportf(h.name.Pos(), format, args...)
}

// lowerInitial lowercases the leading word of a Go name, keeping initialisms
// whole: CancelTransfer → cancelTransfer, APIKeysList → apiKeysList.
func lowerInitial(name string) string {
	runes := []rune(name)

	upper := 0
	for upper < len(runes) && unicode.IsUpper(runes[upper]) {
		upper++
	}

	switch {
	case upper == len(runes):
		return strings.ToLower(name)
	case upper > 1:
		upper-- // the last capital starts the next word
	}

	for i := range upper {
		runes[i] = unicode.ToLower(runes[i])
	}

	return string(runes)
}

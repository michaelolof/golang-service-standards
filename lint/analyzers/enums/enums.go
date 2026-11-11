// Package enums checks the enumx enums in internal/domain against DOM-6. It
// follows enumx's own declaration style and could move into enumx.
//
//   - The type is string-kinded, and its fallback is nil, so "" is invalid.
//   - The package declares Parse<T>(string) (T, error), and T has
//     Validate() error and Oneof() string.
//   - Each variant is a variable named <Type><Variant>, with an UPPER_SNAKE
//     wire value.
//
// Whether Oneof sorts is checked in review: it calls a sorting helper.
package enums

import (
	"go/ast"
	"go/constant"
	"go/types"
	"regexp"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks enumx enums.
var Analyzer = &analysis.Analyzer{
	Name: "enums",
	Doc:  "checks that each enumx enum in domain has a nil fallback, Parse, Validate and Oneof, and well-named UPPER_SNAKE variants",
	Run:  run,
}

var upperSnake = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

// enum is one enumx.New declaration.
type enum struct {
	typ    *types.Named
	setter types.Object
	// lower collects the wire values that aren't UPPER_SNAKE. They're reported
	// once, at the type, so an enum whose values are frozen on an existing scope
	// takes one //nolint.
	lower *[]string
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !where.Within(pkg.Rel, "internal/domain") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	var enums []enum

	for _, spec := range valueSpecs(pass) {
		if e, ok := declaration(pass, spec); ok {
			enums = append(enums, e)
		}
	}

	for _, e := range enums {
		checkMethods(pass, e.typ)
	}

	for _, spec := range valueSpecs(pass) {
		checkVariant(pass, spec, enums)
	}

	for _, e := range enums {
		if len(*e.lower) > 0 {
			pass.Reportf(e.typ.Obj().Pos(), "%s's wire values are UPPER_SNAKE: %s (DOM-6)", e.typ.Obj().Name(), strings.Join(*e.lower, ", "))
		}
	}

	return nil, nil
}

func valueSpecs(pass *analysis.Pass) []*ast.ValueSpec {
	var specs []*ast.ValueSpec

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			if gen, ok := decl.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if vs, ok := spec.(*ast.ValueSpec); ok && len(vs.Values) == 1 {
						specs = append(specs, vs)
					}
				}
			}
		}
	}

	return specs
}

// declaration reads `var Ts, setT, _ = enumx.New[T, M](nil)`, and reports a
// fallback that isn't nil or a type that isn't string-kinded.
func declaration(pass *analysis.Pass, spec *ast.ValueSpec) (enum, bool) {
	call, ok := ast.Unparen(spec.Values[0]).(*ast.CallExpr)
	if !ok || !where.Calls(pass, call, where.Enumx, "New") {
		return enum{}, false
	}

	sig, ok := pass.TypesInfo.TypeOf(call.Fun).(*types.Signature)
	if !ok || sig.Results().Len() < 2 {
		return enum{}, false
	}

	setter, _ := sig.Results().At(1).Type().(*types.Signature)
	if setter == nil || setter.Results().Len() != 1 {
		return enum{}, false
	}

	named, ok := types.Unalias(setter.Results().At(0).Type()).(*types.Named)
	if !ok || named.Obj().Pkg() != pass.Pkg {
		return enum{}, false
	}

	if basic, ok := named.Underlying().(*types.Basic); !ok || basic.Kind() != types.String {
		pass.Reportf(named.Obj().Pos(), "%s is an enum, so it's string-kinded (DOM-6)", named.Obj().Name())
	}

	if len(call.Args) != 1 || !pass.TypesInfo.Types[call.Args[0]].IsNil() {
		pass.Reportf(call.Pos(), "an enum's fallback is nil, so the zero value is invalid: enumx.New[%s, …](nil) (DOM-6)", named.Obj().Name())
	}

	e := enum{typ: named, lower: new([]string)}
	if len(spec.Names) > 1 && spec.Names[1].Name != "_" {
		e.setter = pass.TypesInfo.Defs[spec.Names[1]]
	}

	return e, true
}

// checkMethods reports a missing Parse<T>, Validate or Oneof.
func checkMethods(pass *analysis.Pass, named *types.Named) {
	name := named.Obj().Name()
	pos := named.Obj().Pos()
	errType := types.Universe.Lookup("error").Type()

	parse, _ := pass.Pkg.Scope().Lookup("Parse" + name).(*types.Func)
	if parse == nil || !signatureIs(parse.Signature(), []types.Type{types.Typ[types.String]}, []types.Type{named, errType}) {
		pass.Reportf(pos, "%s needs Parse%s(s string) (%s, error), the one way in from untrusted text (DOM-6)", name, name, name)
	}

	methods := types.NewMethodSet(named)

	if !hasMethod(methods, "Validate", nil, []types.Type{errType}) {
		pass.Reportf(pos, "%s needs a Validate() error method (DOM-6)", name)
	}

	if !hasMethod(methods, "Oneof", nil, []types.Type{types.Typ[types.String]}) {
		pass.Reportf(pos, "%s needs a Oneof() string method, built from a sorted helper (DOM-6)", name)
	}
}

// checkVariant reports a variant whose name or wire value doesn't follow DOM-6:
// `var TransferStatusPending = setTransferStatus("PENDING", nil)`.
func checkVariant(pass *analysis.Pass, spec *ast.ValueSpec, enums []enum) {
	call, ok := ast.Unparen(spec.Values[0]).(*ast.CallExpr)
	if !ok || len(spec.Names) != 1 || len(call.Args) == 0 {
		return
	}

	fn, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return
	}

	for _, e := range enums {
		if e.setter == nil || pass.TypesInfo.Uses[fn] != e.setter {
			continue
		}

		typeName := e.typ.Obj().Name()
		name := spec.Names[0].Name

		if !strings.HasPrefix(name, typeName) || name == typeName {
			pass.Reportf(spec.Names[0].Pos(), "an enum variable is <Type><Variant>, such as %sPending: rename %s (DOM-6)", typeName, name)
		}

		value := pass.TypesInfo.Types[call.Args[0]].Value
		if value != nil && value.Kind() == constant.String && !upperSnake.MatchString(constant.StringVal(value)) {
			*e.lower = append(*e.lower, value.ExactString())
		}
	}
}

func hasMethod(methods *types.MethodSet, name string, params, results []types.Type) bool {
	sel := methods.Lookup(nil, name)
	if sel == nil {
		return false
	}

	sig, ok := sel.Type().(*types.Signature)

	return ok && signatureIs(sig, params, results)
}

func signatureIs(sig *types.Signature, params, results []types.Type) bool {
	if sig.Params().Len() != len(params) || sig.Results().Len() != len(results) || sig.Variadic() {
		return false
	}

	for i, p := range params {
		if !types.Identical(sig.Params().At(i).Type(), p) {
			return false
		}
	}

	for i, r := range results {
		if !types.Identical(sig.Results().At(i).Type(), r) {
			return false
		}
	}

	return true
}

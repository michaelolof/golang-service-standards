// Package services checks how a capability's Service is built (ABS-5). A
// package under internal/core that declares a type named Service:
//
//   - builds it with exactly one function, func New(d Deps) (*Service, error),
//     where Deps is a struct in the same package;
//   - gives Service no exported fields;
//   - gives Service no With… methods.
//
// Whether a capability should have a service at all is for review (HTTP-3).
package services

import (
	"go/ast"
	"go/types"
	"regexp"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks Service construction.
var Analyzer = &analysis.Analyzer{
	Name: "services",
	Doc:  "checks that a core Service is built only by New(Deps) (*Service, error), with no exported fields or With… methods",
	Run:  run,
}

var mutator = regexp.MustCompile(`^With([A-Z]|$)`)

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !where.Within(pkg.Rel, "internal/core") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	service, _ := pass.Pkg.Scope().Lookup("Service").(*types.TypeName)
	if service == nil {
		return nil, nil
	}

	named, ok := service.Type().(*types.Named)
	if !ok {
		return nil, nil
	}

	checkFields(pass, named)
	checkConstructors(pass, named)

	for method := range named.Methods() {
		if mutator.MatchString(method.Name()) {
			pass.Reportf(method.Pos(), "a service is configured only through New(Deps): remove %s (ABS-5)", method.Name())
		}
	}

	return nil, nil
}

func checkFields(pass *analysis.Pass, service *types.Named) {
	st, ok := service.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for field := range st.Fields() {
		if field.Exported() {
			pass.Reportf(field.Pos(), "Service has no exported fields: pass %s through Deps (ABS-5)", field.Name())
		}
	}
}

// checkConstructors reports a missing or misshapen New, and any other function
// that returns a Service.
func checkConstructors(pass *analysis.Pass, service *types.Named) {
	found := false

	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil {
				continue
			}

			obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
			if !ok || !returnsService(obj.Signature(), service) {
				continue
			}

			if fn.Name.Name != "New" {
				pass.Reportf(fn.Name.Pos(), "a service is built only by New(Deps): remove %s (ABS-5)", fn.Name.Name)
				continue
			}

			found = true

			if !newShape(pass, obj.Signature(), service) {
				pass.Reportf(fn.Name.Pos(), "a service's constructor is func New(d Deps) (*Service, error), and it checks the required dependencies (ABS-5)")
			}
		}
	}

	if !found {
		pass.Reportf(service.Obj().Pos(), "Service needs func New(d Deps) (*Service, error) (ABS-5)")
	}
}

func returnsService(sig *types.Signature, service *types.Named) bool {
	for v := range sig.Results().Variables() {
		t := v.Type()
		if ptr, ok := t.(*types.Pointer); ok {
			t = ptr.Elem()
		}

		if types.Identical(t, service) {
			return true
		}
	}

	return false
}

// newShape says whether sig is func(Deps) (*Service, error), with Deps a struct
// declared in the same package.
func newShape(pass *analysis.Pass, sig *types.Signature, service *types.Named) bool {
	if sig.Params().Len() != 1 || sig.Results().Len() != 2 || sig.Variadic() {
		return false
	}

	deps, ok := types.Unalias(sig.Params().At(0).Type()).(*types.Named)
	if !ok || deps.Obj().Name() != "Deps" || deps.Obj().Pkg() != pass.Pkg {
		return false
	}

	if _, ok := deps.Underlying().(*types.Struct); !ok {
		return false
	}

	ptr, ok := sig.Results().At(0).Type().(*types.Pointer)

	return ok && types.Identical(ptr.Elem(), service) && where.IsError(sig.Results().At(1).Type())
}

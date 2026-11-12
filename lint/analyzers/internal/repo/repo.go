// Package repo finds a repository package and its Repo type (DATA-1), and the
// verb a method's name starts with (DATA-3).
package repo

import (
	"go/types"
	"strings"
	"unicode"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// IsAggregate says whether pkg is a repository for one aggregate:
// internal/repository/<aggregate>, and not a test package.
func IsAggregate(pkg where.Package) bool {
	return strings.Count(pkg.Rel, "/") == 2 && where.Within(pkg.Rel, "internal/repository") &&
		!where.IsTestInfra(pkg.Rel, pkg.Name)
}

// Type returns the package's Repo type, or nil.
func Type(pass *analysis.Pass) *types.Named {
	obj, _ := pass.Pkg.Scope().Lookup("Repo").(*types.TypeName)
	if obj == nil {
		return nil
	}

	named, _ := obj.Type().(*types.Named)

	return named
}

// IsRepo says whether t, or what it points to, is the Repo type of a
// repository for one aggregate in module.
func IsRepo(t types.Type, module string) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Name() != "Repo" || named.Obj().Pkg() == nil {
		return false
	}

	pkg := named.Obj().Pkg()

	rel, ok := where.Package{Module: module}.Import(pkg.Path())

	return ok && IsAggregate(where.Package{Module: module, Rel: rel, Name: pkg.Name()})
}

// Verb returns the leading word of a MixedCaps method name: Get in GetByID.
func Verb(name string) string {
	for i, r := range name {
		if i > 0 && unicode.IsUpper(r) {
			return name[:i]
		}
	}

	return name
}

// Methods returns Repo's exported methods, on a value or pointer receiver.
func Methods(named *types.Named) []*types.Func {
	var methods []*types.Func

	for method := range named.Methods() {
		if method.Exported() {
			methods = append(methods, method)
		}
	}

	return methods
}

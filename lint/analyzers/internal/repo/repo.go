// Package repo finds a repository package and its Repo type (DATA-1).
package repo

import (
	"go/types"
	"strings"

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

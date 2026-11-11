package where

import (
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/types/typeutil"
)

// The libraries the standard is built on.
const (
	Errx  = "github.com/michaelolof/errx"
	Gofi  = "github.com/michaelolof/gofi"
	Enumx = "github.com/michaelolof/enumx"
)

// IsNamed says whether t, or what it points to, is the type pkgPath.name.
func IsNamed(t types.Type, pkgPath, name string) bool {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}

	obj := named.Obj()

	return obj.Name() == name && obj.Pkg() != nil && obj.Pkg().Path() == pkgPath
}

// IsContext says whether t is context.Context.
func IsContext(t types.Type) bool {
	return IsNamed(t, "context", "Context")
}

// IsError says whether t is the error interface itself.
func IsError(t types.Type) bool {
	return types.Identical(t, types.Universe.Lookup("error").Type())
}

// Callee returns the function or method call calls, or nil for a call through
// a variable, a builtin or a conversion.
func Callee(pass *analysis.Pass, call *ast.CallExpr) *types.Func {
	fn, _ := typeutil.Callee(pass.TypesInfo, call).(*types.Func)

	return fn
}

// IsFunc says whether fn is the package-level function pkgPath.name.
func IsFunc(fn *types.Func, pkgPath, name string) bool {
	if fn == nil || fn.Pkg() == nil || fn.Name() != name {
		return false
	}

	sig, _ := fn.Type().(*types.Signature)

	return sig != nil && sig.Recv() == nil && fn.Pkg().Path() == pkgPath
}

// Calls says whether call calls the package-level function pkgPath.name.
func Calls(pass *analysis.Pass, call *ast.CallExpr, pkgPath, name string) bool {
	return IsFunc(Callee(pass, call), pkgPath, name)
}

// TypeName returns the name of t's type, through a pointer, or "" when it has none.
func TypeName(t types.Type) string {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	if named, ok := types.Unalias(t).(*types.Named); ok {
		return named.Obj().Name()
	}

	return ""
}

// Walk calls visit for each named type t mentions, without looking inside
// named types, and stops when visit returns true.
func Walk(t types.Type, visit func(*types.TypeName) bool) {
	walk(t, map[types.Type]bool{}, visit)
}

func walk(t types.Type, seen map[types.Type]bool, visit func(*types.TypeName) bool) bool {
	if t == nil || seen[t] {
		return false
	}

	seen[t] = true

	switch typ := t.(type) {
	case *types.Alias:
		return visit(typ.Obj()) || walk(types.Unalias(typ), seen, visit)
	case *types.Named:
		if visit(typ.Obj()) {
			return true
		}

		for arg := range typ.TypeArgs().Types() {
			if walk(arg, seen, visit) {
				return true
			}
		}
	case *types.Pointer:
		return walk(typ.Elem(), seen, visit)
	case *types.Slice:
		return walk(typ.Elem(), seen, visit)
	case *types.Array:
		return walk(typ.Elem(), seen, visit)
	case *types.Chan:
		return walk(typ.Elem(), seen, visit)
	case *types.Map:
		return walk(typ.Key(), seen, visit) || walk(typ.Elem(), seen, visit)
	case *types.Signature:
		for v := range typ.Params().Variables() {
			if walk(v.Type(), seen, visit) {
				return true
			}
		}

		for v := range typ.Results().Variables() {
			if walk(v.Type(), seen, visit) {
				return true
			}
		}
	case *types.Struct:
		for field := range typ.Fields() {
			if field.Exported() && walk(field.Type(), seen, visit) {
				return true
			}
		}
	}

	return false
}

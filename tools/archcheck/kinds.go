package main

import (
	"go/ast"
	"go/types"
)

const (
	errx      = "github.com/michaelolof/errx"
	kindsHome = "internal/foundation/errkinds"
)

// creators are the errx functions that give an error a kind.
var creators = map[string]bool{"NewKind": true, "NewKindf": true, "WrapKind": true, "WrapKindf": true}

// kindUses counts where one kind appears outside errkinds, in code that isn't a test.
type kindUses struct {
	created, checked, other int
}

// checkKinds reports a kind in foundation/errkinds that nothing outside it
// uses, that nothing creates, or that nothing checks (ERR-5). A kind passed
// anywhere other than errx's creating functions and IsKind, such as into a
// mapping table, counts as both. Whether a check is a branch, and not a choice
// of message, is for review.
func checkKinds(m *module) ([]problem, error) {
	kinds := map[types.Object]*kindUses{}

	var home []*ast.Ident

	for _, p := range m.code {
		rel, _ := m.relPkg(p.PkgPath)
		if rel != kindsHome {
			continue
		}

		for ident, obj := range p.TypesInfo.Defs {
			if v, ok := obj.(*types.Var); ok && v.Parent() == p.Types.Scope() && isErrxKind(v.Type()) {
				kinds[obj] = &kindUses{}
				home = append(home, ident)
			}
		}
	}

	for _, p := range m.code {
		rel, _ := m.relPkg(p.PkgPath)
		if rel == kindsHome || isTestPackage(rel, p.Name) {
			continue
		}

		for _, file := range p.Syntax {
			countKindUses(p.TypesInfo, file, kinds)
		}
	}

	var problems []problem

	for _, p := range m.code {
		if rel, _ := m.relPkg(p.PkgPath); rel != kindsHome {
			continue
		}

		for _, ident := range home {
			uses := kinds[p.TypesInfo.Defs[ident]]
			if uses == nil {
				continue
			}

			switch {
			case uses.created+uses.checked+uses.other == 0:
				problems = append(problems, m.at(p.Fset, ident.Pos(), "nothing outside errkinds uses %s: delete it (ERR-5)", ident.Name))
			case uses.checked+uses.other == 0:
				problems = append(problems, m.at(p.Fset, ident.Pos(), "nothing checks %s with errx.IsKind: a kind exists only for code that branches on it (ERR-5)", ident.Name))
			case uses.created+uses.other == 0:
				problems = append(problems, m.at(p.Fset, ident.Pos(), "nothing creates an error of kind %s, so checking for it never matches (ERR-5)", ident.Name))
			}
		}
	}

	return problems, nil
}

// countKindUses counts each use of a kind in file by what it's passed to.
func countKindUses(info *types.Info, file *ast.File, kinds map[types.Object]*kindUses) {
	inCall := map[*ast.Ident]string{}

	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		name := errxFunc(info, call)

		for _, arg := range call.Args {
			if ident := kindIdent(arg); ident != nil {
				inCall[ident] = name
			}
		}

		return true
	})

	ast.Inspect(file, func(n ast.Node) bool {
		ident, ok := n.(*ast.Ident)
		if !ok {
			return true
		}

		uses := kinds[info.Uses[ident]]
		if uses == nil {
			return true
		}

		switch name := inCall[ident]; {
		case creators[name]:
			uses.created++
		case name == "IsKind":
			uses.checked++
		default:
			uses.other++
		}

		return true
	})
}

// kindIdent returns the identifier an argument names: errkinds.X gives X.
func kindIdent(expr ast.Expr) *ast.Ident {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return e
	case *ast.SelectorExpr:
		return e.Sel
	}

	return nil
}

// errxFunc returns the name of the errx function call calls, or "".
func errxFunc(info *types.Info, call *ast.CallExpr) string {
	var ident *ast.Ident

	switch fun := ast.Unparen(call.Fun).(type) {
	case *ast.SelectorExpr:
		ident = fun.Sel
	case *ast.Ident:
		ident = fun
	default:
		return ""
	}

	fn, ok := info.Uses[ident].(*types.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != errx {
		return ""
	}

	return fn.Name()
}

// isErrxKind says whether t is errx's kind type, the type errx.Kind returns.
func isErrxKind(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)

	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == errx && named.Obj().Name() == "errKind"
}

// Package names checks the names the standard bans or fixes, where revive and
// staticcheck can't express the rule.
//
//   - A package named for a kind of helper: any name ending in util, utils,
//     helper or helpers (LAY-9). revive's package-naming bans the exact names.
//   - A file named helper(s).go, util(s).go, common.go or types.go (LAY-10).
//   - An interface named I<Name>, or any type ending in Iface, Interface, Impl,
//     Adapter, Manager or Helper (ABS-3).
//   - In core, an input type ending in Params, Param or Arg, instead of <Op>Args (ABS-6).
//   - A getter named GetX, a constructor named Build… or Make…, and an error
//     variable that isn't err or <verb>Err (STY-2).
//
// Test packages are left alone while restructuring tests is paused (TEST-14).
package names

import (
	"go/ast"
	"go/token"
	"go/types"
	"regexp"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks names.
var Analyzer = &analysis.Analyzer{
	Name: "names",
	Doc:  "checks the package, file, type, getter, constructor and error-variable names the standard fixes",
	Run:  run,
}

var (
	bannedFiles = map[string]bool{
		"helper.go": true, "helpers.go": true, "util.go": true, "utils.go": true,
		"common.go": true, "types.go": true,
	}
	bannedTypeSuffixes = []string{"Iface", "Interface", "Impl", "Adapter", "Manager", "Helper"}
	interfacePrefix    = regexp.MustCompile(`^I[A-Z][a-z]`)
	getter             = regexp.MustCompile(`^Get[A-Z]`)
	constructor        = regexp.MustCompile(`^(Build|Make)([A-Z]|$)`)
)

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	checkPackageName(pass, pkg)

	for _, file := range where.Files(pass) {
		if name := where.FileName(pass, file); bannedFiles[name] {
			pass.Reportf(file.Name.Pos(), "%s says what kind of code it holds, not what it does: name the file after its type or use case (LAY-10)", name)
		}

		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				checkTypes(pass, pkg, d)
			case *ast.FuncDecl:
				checkFunc(pass, d)
			}
		}
	}

	return nil, nil
}

func checkPackageName(pass *analysis.Pass, pkg where.Package) {
	for _, suffix := range []string{"util", "utils", "helper", "helpers"} {
		if strings.HasSuffix(pkg.Name, suffix) && pkg.Name != suffix {
			pass.Reportf(where.PackageClause(pass, "doc.go"), "package %s is named for a kind of helper: name it for what it provides, under foundation/ if several packages use it (LAY-9)", pkg.Name)
			return
		}
	}
}

func checkTypes(pass *analysis.Pass, pkg where.Package, decl *ast.GenDecl) {
	if decl.Tok != token.TYPE {
		return
	}

	for _, spec := range decl.Specs {
		ts, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		name := ts.Name.Name
		_, isInterface := ts.Type.(*ast.InterfaceType)

		switch {
		case isInterface && interfacePrefix.MatchString(name):
			pass.Reportf(ts.Name.Pos(), "name the interface %s by its role, without the I prefix: -er, Store or a domain noun (ABS-3)", name)
		case hasAnySuffix(name, bannedTypeSuffixes):
			pass.Reportf(ts.Name.Pos(), "name %s by its role, not by how it's built: -er, Store, Repo or a domain noun (ABS-3)", name)
		case where.Within(pkg.Rel, "internal/core") && isUseCaseInput(pass, ts) &&
			(strings.HasSuffix(name, "Params") || strings.HasSuffix(name, "Param") || strings.HasSuffix(name, "Arg")):
			pass.Reportf(ts.Name.Pos(), "a use case takes <Op>Args and returns <Op>Result: rename %s (ABS-6)", name)
		}

		if iface, ok := ts.Type.(*ast.InterfaceType); ok {
			checkInterfaceGetters(pass, iface)
		}
	}
}

// isUseCaseInput says whether ts is the input of a use case: a parameter of an
// exported method on the package's Service (ABS-6).
func isUseCaseInput(pass *analysis.Pass, ts *ast.TypeSpec) bool {
	input := pass.TypesInfo.Defs[ts.Name]

	service, _ := pass.Pkg.Scope().Lookup("Service").(*types.TypeName)
	if input == nil || service == nil {
		return false
	}

	named, ok := service.Type().(*types.Named)
	if !ok {
		return false
	}

	for method := range named.Methods() {
		if !method.Exported() {
			continue
		}

		for param := range method.Signature().Params().Variables() {
			if where.TypeName(param.Type()) == input.Name() && types.Identical(param.Type(), input.Type()) {
				return true
			}
		}
	}

	return false
}

func checkInterfaceGetters(pass *analysis.Pass, iface *ast.InterfaceType) {
	for _, method := range iface.Methods.List {
		ft, ok := method.Type.(*ast.FuncType)
		if !ok {
			continue
		}

		for _, name := range method.Names {
			if getter.MatchString(name.Name) && ft.Params.NumFields() == 0 {
				pass.Reportf(name.Pos(), "a getter is named for what it returns: %s, not %s (STY-2)", strings.TrimPrefix(name.Name, "Get"), name.Name)
			}
		}
	}
}

func checkFunc(pass *analysis.Pass, fn *ast.FuncDecl) {
	name := fn.Name.Name

	if fn.Recv != nil && getter.MatchString(name) && fn.Type.Params.NumFields() == 0 {
		pass.Reportf(fn.Name.Pos(), "a getter is named for what it returns: %s, not %s (STY-2)", strings.TrimPrefix(name, "Get"), name)
	}

	if fn.Recv == nil && fn.Name.IsExported() && constructor.MatchString(name) && buildsOwnType(pass, fn) {
		pass.Reportf(fn.Name.Pos(), "a constructor is New, or New<Type> when the package has several types: rename %s (STY-2)", name)
	}

	if fn.Body != nil {
		checkErrorVariables(pass, fn.Body)
	}
}

// buildsOwnType says whether fn's first result is a type declared in its own package.
func buildsOwnType(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	obj, ok := pass.TypesInfo.Defs[fn.Name].(*types.Func)
	if !ok {
		return false
	}

	results := obj.Signature().Results()
	if results.Len() == 0 {
		return false
	}

	t := results.At(0).Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, ok := types.Unalias(t).(*types.Named)

	return ok && named.Obj().Pkg() == pass.Pkg
}

// checkErrorVariables reports a local variable of type error with another name.
func checkErrorVariables(pass *analysis.Pass, body *ast.BlockStmt) {
	check := func(ident *ast.Ident) {
		if ident == nil || ident.Name == "_" || ident.Name == "err" || strings.HasSuffix(ident.Name, "Err") {
			return
		}

		if obj, ok := pass.TypesInfo.Defs[ident].(*types.Var); ok && where.IsError(obj.Type()) {
			pass.Reportf(ident.Pos(), "an error variable is err, or <verb>Err for a second one in scope: rename %s (STY-2)", ident.Name)
		}
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, lhs := range node.Lhs {
					ident, _ := lhs.(*ast.Ident)
					check(ident)
				}
			}
		case *ast.ValueSpec:
			for _, name := range node.Names {
				check(name)
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				key, _ := node.Key.(*ast.Ident)
				value, _ := node.Value.(*ast.Ident)
				check(key)
				check(value)
			}
		}

		return true
	})
}

func hasAnySuffix(name string, suffixes []string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}

	return false
}

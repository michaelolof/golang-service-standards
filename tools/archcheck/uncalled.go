package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"golang.org/x/tools/go/callgraph/rta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

// checkUncalledTestCode reports each function and method in test code that no
// test reaches (TEST-19). Test code is a _test.go file, or a test package: one
// named …test or …tests, or anything under internal/testutils (TEST-13).
//
// It works like deadcode -test: rapid type analysis from the entry points of
// every binary the module builds, each test binary included, with every kind's
// tag set, so a helper only a flow calls counts as called. Two kinds of
// function are left out, because deleting them would break the build or the
// tests: a method a type needs for an interface it's converted to, and the
// tests, benchmarks, fuzz tests and examples go test runs. Generated files
// aren't read. Unused types, fields and constants are for review.
func checkUncalledTestCode(m *module) ([]problem, error) {
	prog, pkgs := ssautil.AllPackages(m.loaded, ssa.InstantiateGenerics)
	prog.Build()

	reached := reachable(prog, pkgs)
	needed := m.neededByInterfaces(prog, pkgs)

	var problems []problem

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) {
		if ast.IsGenerated(file) || !isTestCode(m, p, rel) {
			return
		}

		testFile := strings.HasSuffix(rel, "_test.go")

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name == "init" || fn.Name.Name == "_" || (testFile && fn.Recv == nil && runByGoTest(fn.Name.Name)) {
				continue
			}

			pos := p.Fset.Position(fn.Name.Pos())
			if reached[pos] || needed[pos] {
				continue
			}

			problems = append(problems, m.at(p.Fset, fn.Name.Pos(), "%s is test code nothing calls: delete it (TEST-19)", funcName(fn)))
		}
	})

	return problems, nil
}

// reachable returns the position of every function that rapid type analysis
// reaches from the init and main of each main package, test binaries included.
// A test build compiles its own copy of a package, so a function counts as
// reached when any copy is.
func reachable(prog *ssa.Program, pkgs []*ssa.Package) map[token.Position]bool {
	var roots []*ssa.Function

	for _, main := range ssautil.MainPackages(pkgs) {
		roots = append(roots, main.Func("init"), main.Func("main"))
	}

	reached := map[token.Position]bool{}
	if len(roots) == 0 {
		return reached
	}

	for fn := range rta.Analyze(roots, false).Reachable {
		if fn.Pos().IsValid() {
			reached[prog.Fset.Position(fn.Pos())] = true
		}
	}

	return reached
}

// neededByInterfaces returns the position of every method that a type of the
// module needs for an interface it's converted to, explicitly or not, or for
// the constraint of a type parameter it's passed as. Deleting one breaks the
// build, so it's never reported, even when nothing calls it.
func (m *module) neededByInterfaces(prog *ssa.Program, pkgs []*ssa.Package) map[token.Position]bool {
	needed := map[token.Position]bool{}
	done := map[string]bool{}

	need := func(concrete, target types.Type) {
		iface, ok := target.Underlying().(*types.Interface)
		if !ok || iface.NumMethods() == 0 || concrete == nil || types.IsInterface(concrete) {
			return
		}

		pair := types.TypeString(concrete, nil) + " -> " + types.TypeString(target, nil)
		if done[pair] {
			return
		}

		done[pair] = true
		methods := prog.MethodSets.MethodSet(concrete)

		for i := range iface.NumMethods() {
			want := iface.Method(i)
			if sel := methods.Lookup(want.Pkg(), want.Name()); sel != nil {
				needed[prog.Fset.Position(sel.Obj().Pos())] = true
			}
		}
	}

	for _, fn := range m.moduleFunctions(prog, pkgs) {
		for _, block := range fn.Blocks {
			for _, instr := range block.Instrs {
				if mi, ok := instr.(*ssa.MakeInterface); ok {
					need(mi.X.Type(), mi.Type())
				}
			}
		}
	}

	for _, p := range m.all {
		info := p.TypesInfo

		// var _ I = (*T)(nil) asserts T without converting anything at run time.
		for _, file := range p.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				if spec, ok := n.(*ast.ValueSpec); ok && spec.Type != nil {
					for _, value := range spec.Values {
						need(info.TypeOf(value), info.TypeOf(spec.Type))
					}
				}

				return true
			})
		}

		for ident, inst := range info.Instances {
			params := typeParams(info.Uses[ident])

			for i := range min(params.Len(), inst.TypeArgs.Len()) {
				need(inst.TypeArgs.At(i), params.At(i).Constraint())
			}
		}
	}

	return needed
}

// moduleFunctions returns every function the module declares, with its
// methods and the function literals inside them.
func (m *module) moduleFunctions(prog *ssa.Program, pkgs []*ssa.Package) []*ssa.Function {
	var found []*ssa.Function

	var add func(fn *ssa.Function)
	add = func(fn *ssa.Function) {
		if fn == nil {
			return
		}

		found = append(found, fn)

		for _, anon := range fn.AnonFuncs {
			add(anon)
		}
	}

	for _, pkg := range pkgs {
		if pkg == nil {
			continue
		}

		if _, ok := m.relPkg(pkg.Pkg.Path()); !ok {
			continue
		}

		for _, member := range pkg.Members {
			switch mem := member.(type) {
			case *ssa.Function:
				add(mem)
			case *ssa.Type:
				if named, ok := mem.Type().(*types.Named); !ok || named.TypeParams().Len() > 0 || types.IsInterface(named) {
					continue // a generic type's methods are built for each instance
				}

				for _, t := range []types.Type{mem.Type(), types.NewPointer(mem.Type())} {
					methods := prog.MethodSets.MethodSet(t)
					for i := range methods.Len() {
						add(prog.MethodValue(methods.At(i)))
					}
				}
			}
		}
	}

	return found
}

// typeParams returns the type parameters of a generic function or type.
func typeParams(obj types.Object) *types.TypeParamList {
	switch t := obj.(type) {
	case *types.Func:
		return t.Signature().TypeParams()
	case *types.TypeName:
		if named, ok := t.Type().(*types.Named); ok {
			return named.TypeParams()
		}
	}

	return nil
}

// isTestCode says whether a file is test code: a _test.go file, or a file of a
// test package (TEST-13).
func isTestCode(m *module, p *packages.Package, rel string) bool {
	if strings.HasSuffix(rel, "_test.go") {
		return true
	}

	pkgRel, ok := m.relPkg(p.PkgPath)

	return ok && isTestPackage(pkgRel, p.Name)
}

// runByGoTest says whether go test runs a function by its name: a test, a
// benchmark, a fuzz test, an example or TestMain.
func runByGoTest(name string) bool {
	for _, prefix := range []string{"Test", "Benchmark", "Fuzz", "Example"} {
		if isTestFunc(name, prefix) {
			return true
		}
	}

	return false
}

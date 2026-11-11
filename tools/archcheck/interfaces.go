package main

import (
	"cmp"
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

// iface is one interface declared in the module.
type iface struct {
	pkg  *packages.Package
	rel  string
	obj  *types.TypeName
	typ  *types.Interface
	name string // pkg.Name, such as transfer.Store
}

// interfaces returns the interfaces the checks look at, in code that isn't a test.
func (m *module) interfaces() []iface {
	var found []iface

	for _, p := range m.code {
		found = append(found, m.interfacesIn(p)...)
	}

	return found
}

// interfacesIn returns the interfaces p declares at package level under
// internal/, with at least one method and no type parameters. Test packages,
// and gofi's Deps in internal/http/deps, aren't among them (ABS-1).
func (m *module) interfacesIn(p *packages.Package) []iface {
	rel, _ := m.relPkg(p.PkgPath)
	if !within(rel, "internal") || within(rel, "internal/http/deps") || isTestPackage(rel, p.Name) {
		return nil
	}

	var found []iface

	scope := p.Types.Scope()

	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || obj.IsAlias() {
			continue
		}

		named, ok := obj.Type().(*types.Named)
		if !ok || named.TypeParams().Len() > 0 {
			continue
		}

		if typ, ok := named.Underlying().(*types.Interface); ok && typ.NumMethods() > 0 {
			found = append(found, iface{pkg: p, rel: rel, obj: obj, typ: typ, name: p.Name + "." + name})
		}
	}

	return found
}

// concrete returns every named type that isn't an interface and has no type
// parameters, declared at package level in pkgs.
func concrete(pkgs []*packages.Package) []*types.Named {
	var found []*types.Named

	for _, p := range pkgs {
		found = append(found, concreteIn(p.Types)...)
	}

	return found
}

func concreteIn(pkg *types.Package) []*types.Named {
	var found []*types.Named

	scope := pkg.Scope()

	for _, name := range scope.Names() {
		obj, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || obj.IsAlias() {
			continue
		}

		if named, ok := obj.Type().(*types.Named); ok && named.TypeParams().Len() == 0 && !types.IsInterface(named) {
			found = append(found, named)
		}
	}

	return found
}

func implements(t *types.Named, i *types.Interface) bool {
	return types.Implements(t, i) || types.Implements(types.NewPointer(t), i)
}

// checkInterfaces reports an interface nothing implements, a port nothing
// outside internal/ports uses, a job interface nothing outside internal/jobs
// uses, a foundation interface nothing uses, and two interfaces with the same
// methods (ABS-1, JOB-1). iface's unused check covers use inside a consumer's
// package.
func checkInterfaces(m *module) ([]problem, error) {
	ifaces := m.interfaces()
	implemented := m.implemented(ifaces)
	used, usedOutside := m.uses()

	var problems []problem

	for _, i := range ifaces {
		if !implemented[key(i.obj)] {
			problems = append(problems, m.at(i.pkg.Fset, i.obj.Pos(), "nothing implements %s, not even a test fake: delete it until something does (ABS-1)", i.obj.Name()))
		}

		switch {
		case within(i.rel, "internal/ports") && !usedOutside[i.obj]:
			problems = append(problems, m.at(i.pkg.Fset, i.obj.Pos(), "nothing outside internal/ports uses %s: a port is shared by several packages, so move it to its one consumer or delete it (ABS-1)", i.obj.Name()))
		case within(i.rel, "internal/jobs") && !usedOutside[i.obj]:
			problems = append(problems, m.at(i.pkg.Fset, i.obj.Pos(), "nothing outside internal/jobs uses %s: jobs holds the contract other packages share, so delete it until one does (ABS-1, JOB-1)", i.obj.Name()))
		case within(i.rel, "internal/foundation") && !used[i.obj]:
			problems = append(problems, m.at(i.pkg.Fset, i.obj.Pos(), "nothing uses %s: delete it until something does (ABS-1)", i.obj.Name()))
		}
	}

	return append(problems, duplicates(m, ifaces)...), nil
}

// implemented returns the keys of the interfaces something implements: a type
// in the module, tests included, or in a package the module imports, such as a
// driver's pool. A test build of a package type-checks its own copy of it, so
// each type is checked against every copy of an interface.
func (m *module) implemented(ifaces []iface) map[string]bool {
	copies := map[string][]*types.Interface{}

	for _, p := range m.all {
		for _, i := range m.interfacesIn(p) {
			copies[key(i.obj)] = append(copies[key(i.obj)], i.typ)
		}
	}

	found := map[string]bool{}

	mark := func(candidates []*types.Named) {
		for _, t := range candidates {
			for k, typs := range copies {
				if !found[k] && slices.ContainsFunc(typs, func(i *types.Interface) bool { return implements(t, i) }) {
					found[k] = true
				}
			}
		}
	}

	mark(concrete(m.all))

	if slices.ContainsFunc(ifaces, func(i iface) bool { return !found[key(i.obj)] }) {
		mark(m.importedTypes())
	}

	return found
}

// importedTypes returns the exported named types of every package outside the
// module that the module imports directly.
func (m *module) importedTypes() []*types.Named {
	seen := map[string]bool{}

	var found []*types.Named

	for _, p := range m.code {
		for _, imp := range p.Types.Imports() {
			if _, inModule := m.relPkg(imp.Path()); inModule || seen[imp.Path()] {
				continue
			}

			seen[imp.Path()] = true

			for _, named := range concreteIn(imp) {
				if named.Obj().Exported() {
					found = append(found, named)
				}
			}
		}
	}

	return found
}

// uses returns the type names used anywhere other than their declaration, and
// those used in a package other than their own, in code that isn't a test.
func (m *module) uses() (used, outside map[types.Object]bool) {
	used, outside = map[types.Object]bool{}, map[types.Object]bool{}

	for _, p := range m.code {
		for _, obj := range p.TypesInfo.Uses {
			if tn, ok := obj.(*types.TypeName); ok && tn.Pkg() != nil {
				used[tn] = true
				outside[tn] = outside[tn] || tn.Pkg() != p.Types
			}
		}
	}

	return used, outside
}

// duplicates reports each interface whose method set another package's
// interface already has. Ports, foundation and jobs come first, so the copy
// reported is the one outside the shared home.
func duplicates(m *module, ifaces []iface) []problem {
	home := func(i iface) int {
		switch {
		case within(i.rel, "internal/ports"):
			return 0
		case within(i.rel, "internal/foundation"):
			return 1
		case within(i.rel, "internal/jobs"):
			return 2
		default:
			return 3
		}
	}

	ordered := slices.Clone(ifaces)
	slices.SortStableFunc(ordered, func(a, b iface) int {
		return cmp.Or(cmp.Compare(home(a), home(b)), cmp.Compare(a.rel, b.rel), cmp.Compare(a.obj.Name(), b.obj.Name()))
	})

	first := map[string]iface{}

	var problems []problem

	for _, i := range ordered {
		set := methodSet(i.typ)

		if earlier, ok := first[set]; ok && earlier.pkg != i.pkg {
			problems = append(problems, m.at(i.pkg.Fset, i.obj.Pos(), "%s has the same methods as %s: declare it once, in internal/ports when several packages need it (ABS-1)", i.obj.Name(), earlier.name))
			continue
		}

		first[set] = i
	}

	return problems
}

// methodSet writes an interface's methods, with full package paths, so two
// interfaces compare equal only when every method's signature does.
func methodSet(t *types.Interface) string {
	methods := make([]string, t.NumMethods())

	for i := range t.NumMethods() {
		fn := t.Method(i)
		methods[i] = fn.Name() + types.TypeString(fn.Type(), func(p *types.Package) string { return p.Path() })
	}

	slices.Sort(methods)

	return strings.Join(methods, ";")
}

// checkAssertions reports a type in internal/repository or internal/adapters
// that the module uses as one of its own exported interfaces, declared in
// another package, with no var _ I = (*T)(nil) for that interface anywhere in
// the module (ABS-2). A use is a conversion in code that isn't a test: a struct
// field, an argument, an assignment or a return. A type that only happens to
// have an interface's methods isn't asked for an assertion.
func checkAssertions(m *module) ([]problem, error) {
	asserted := m.assertions()
	exported := map[string]iface{}

	for _, i := range m.interfaces() {
		if i.obj.Exported() {
			exported[key(i.obj)] = i
		}
	}

	var problems []problem

	reported := map[string]bool{}

	for _, c := range m.conversions() {
		i, ok := exported[key(c.to.Obj())]
		pair := key(c.to.Obj()) + " <- " + key(c.from.Obj())

		if !ok || asserted[pair] || reported[pair] || c.from.Obj().Pkg() == c.to.Obj().Pkg() {
			continue
		}

		rel, inModule := m.relPkg(c.from.Obj().Pkg().Path())
		if !inModule || (!within(rel, "internal/repository") && !within(rel, "internal/adapters")) || isTestPackage(rel, c.from.Obj().Pkg().Name()) {
			continue
		}

		p := m.packageOf(c.from.Obj().Pkg())
		if p == nil {
			continue
		}

		reported[pair] = true
		problems = append(problems, m.at(p.Fset, c.from.Obj().Pos(), "%s is used as %s: declare var _ %s = (*%s)(nil) beside it, so a change that breaks it fails here (ABS-2)",
			c.from.Obj().Name(), i.name, i.name, c.from.Obj().Name()))
	}

	return problems, nil
}

// conversion is a value of a concrete type used where an interface is expected.
type conversion struct {
	from, to *types.Named
}

// conversions returns the conversions from a named concrete type to a named
// interface in the module's code that isn't a test.
func (m *module) conversions() []conversion {
	var found []conversion

	for _, p := range m.code {
		info := p.TypesInfo

		add := func(target types.Type, value ast.Expr) {
			to, ok := types.Unalias(target).(*types.Named)
			if !ok || !types.IsInterface(to) {
				return
			}

			if from := namedOf(info.TypeOf(value)); from != nil && !types.IsInterface(from) {
				found = append(found, conversion{from: from, to: to})
			}
		}

		for _, file := range p.Syntax {
			var results *types.Tuple

			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.FuncDecl:
					if fn, ok := info.Defs[node.Name].(*types.Func); ok {
						results = fn.Signature().Results()
					}
				case *ast.CompositeLit:
					convertFields(info, node, add)
				case *ast.CallExpr:
					convertArgs(info, node, add)
				case *ast.AssignStmt:
					if len(node.Lhs) == len(node.Rhs) {
						for i, lhs := range node.Lhs {
							add(info.TypeOf(lhs), node.Rhs[i])
						}
					}
				case *ast.ValueSpec:
					if node.Type != nil {
						for _, value := range node.Values {
							add(info.TypeOf(node.Type), value)
						}
					}
				case *ast.ReturnStmt:
					if results != nil && results.Len() == len(node.Results) {
						for i, value := range node.Results {
							add(results.At(i).Type(), value)
						}
					}
				}

				return true
			})
		}
	}

	return found
}

// convertFields records each keyed field of a struct literal.
func convertFields(info *types.Info, lit *ast.CompositeLit, add func(types.Type, ast.Expr)) {
	t := info.TypeOf(lit)
	if t == nil {
		return
	}

	st, ok := t.Underlying().(*types.Struct)
	if !ok {
		return
	}

	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		name, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		for field := range st.Fields() {
			if field.Name() == name.Name {
				add(field.Type(), kv.Value)
			}
		}
	}
}

// convertArgs records each argument passed to a parameter.
func convertArgs(info *types.Info, call *ast.CallExpr, add func(types.Type, ast.Expr)) {
	sig, ok := info.TypeOf(call.Fun).(*types.Signature)
	if !ok {
		return
	}

	params := sig.Params()

	for i, arg := range call.Args {
		switch {
		case sig.Variadic() && i >= params.Len()-1:
			if slice, ok := params.At(params.Len() - 1).Type().(*types.Slice); ok {
				add(slice.Elem(), arg)
			}
		case i < params.Len():
			add(params.At(i).Type(), arg)
		}
	}
}

// assertions returns every var _ I = expr in the module, tests included, as
// "pkgpath.I <- pkgpath.T".
func (m *module) assertions() map[string]bool {
	found := map[string]bool{}

	for _, p := range m.all {
		for _, file := range p.Syntax {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}

				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok || vs.Type == nil || len(vs.Names) != len(vs.Values) {
						continue
					}

					target, ok := types.Unalias(p.TypesInfo.TypeOf(vs.Type)).(*types.Named)
					if !ok {
						continue
					}

					for i, name := range vs.Names {
						if value := namedOf(p.TypesInfo.TypeOf(vs.Values[i])); name.Name == "_" && value != nil {
							found[key(target.Obj())+" <- "+key(value.Obj())] = true
						}
					}
				}
			}
		}
	}

	return found
}

func namedOf(t types.Type) *types.Named {
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}

	named, _ := types.Unalias(t).(*types.Named)

	return named
}

// key names a type by its package path, so a test build's copy of a package
// matches the package itself.
func key(obj *types.TypeName) string {
	if obj.Pkg() == nil {
		return obj.Name()
	}

	return obj.Pkg().Path() + "." + obj.Name()
}

func (m *module) packageOf(pkg *types.Package) *packages.Package {
	for _, p := range m.code {
		if p.Types == pkg {
			return p
		}
	}

	return nil
}

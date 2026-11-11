// Package layers checks where a package sits and what it imports, against the
// layer table in LAY-6 and LAY-7.
//
//   - Go code lives in cmd/, internal/ and pkg/. The only other Go is the
//     embed shim in db/migrations and the black-box suites in test/ (LAY-1).
//   - Every package under internal/ is in one of LAY-6's layers, and domain,
//     ports and jobs have no subpackages except their test helpers (LAY-6).
//   - ports declares no constructors (LAY-6).
//   - A package imports only the layers its LAY-6 row allows (LAY-7).
//   - Inside adapters and repositories, each SDK is imported only in the adapter
//     for its system, and a repository imports only database drivers (LAY-7).
//   - Test code is imported only by tests and other test packages: a <pkg>test
//     package, anything under internal/testutils, the testing package and
//     testify (TEST-13). depguard bans testify's suite and mock everywhere.
//
// It leaves to depguard what depguard already checks: SDKs outside adapters,
// repositories, cmd and pkg; gofi outside http; internal/config outside cmd; and
// pkg/ importing internal/.
package layers

import (
	"go/ast"
	"strconv"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks package placement and imports against the layer table.
var Analyzer = &analysis.Analyzer{
	Name: "layers",
	Doc:  "checks that packages sit in LAY-6's layers and import only what LAY-7 allows",
	Run:  run,
}

// mayImport is LAY-6's "May import (internal)" column. internal/config is left
// out of every row, because depguard keeps it in cmd.
var mayImport = map[string][]string{
	"foundation": {"foundation"},
	"config":     {"foundation"},
	"domain":     {"foundation"},
	"ports":      {"domain", "foundation"},
	"jobs":       {"domain", "foundation"},
	"core":       {"domain", "ports", "jobs", "foundation", "core"},
	"repository": {"domain", "ports", "jobs", "foundation", "core", "adapters/<name>store"},
	"adapters":   {"domain", "ports", "jobs", "foundation", "core"},
	"http":       {"core", "repository", "domain", "ports", "foundation", "http"},
	"workers":    {"core", "repository", "domain", "ports", "jobs", "foundation"},
}

// flat are the layers that are one package each (LAY-6).
var flat = map[string]bool{"domain": true, "ports": true, "jobs": true}

// perThing are the layers that hold one package per thing: a repository per
// aggregate, an adapter per system, a worker per runtime. Their root holds no
// code. The repos analyzer reports code in internal/repository itself.
var perThing = map[string]bool{"repository": true, "adapters": true, "workers": true}

// moves are the folders LAY-6 says where to put.
var moves = map[string]string{
	"service":    "internal/core",
	"services":   "internal/core",
	"edge":       "internal/http/edge",
	"platform":   "internal/http/platform",
	"schedulers": "internal/workers/schedulers",
	"archival":   "internal/workers/archival",
	"processor":  "internal/workers/processor",
	"testutil":   "internal/testutils",
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok {
		return nil, nil
	}

	layer, item := where.Layer(pkg.Rel)
	internal := where.Within(pkg.Rel, "internal")

	if layer == "" && !internal && !outsideAllowed(pkg.Rel) {
		name := pkg.Rel
		if name == "" {
			name = "the module's root package"
		}

		pass.Reportf(where.PackageClause(pass, "doc.go"), "Go code lives in cmd/, internal/ and pkg/: move %s (LAY-1)", name)
	}

	if internal && !checkPlacement(pass, pkg, layer, item) {
		return nil, nil
	}

	if layer == "ports" && item == "" {
		reportConstructors(pass)
	}

	for _, file := range where.Files(pass) {
		for _, spec := range file.Imports {
			checkImport(pass, pkg, spec)
		}
	}

	return nil, nil
}

// outsideAllowed says whether rel is one of the places outside cmd, internal and
// pkg that may hold Go: the migrations' embed shim, and the black-box suites in
// test/ (TEST-1).
func outsideAllowed(rel string) bool {
	return rel == "db/migrations" || where.Within(rel, "test")
}

// checkPlacement reports a package under internal/ that isn't in a LAY-6 layer,
// that sits below a flat one, or that sits at the root of a layer holding one
// package per thing. It returns false when the package has no place in the
// table, so its imports have no row to be checked against.
func checkPlacement(pass *analysis.Pass, pkg where.Package, layer, item string) bool {
	pos := where.PackageClause(pass, "doc.go")

	switch {
	case layer == "":
		pass.Reportf(pos, "a package directly in internal/ isn't in a LAY-6 layer: move it into one (LAY-6)")
		return false
	case !where.IsLayer(layer):
		if to, ok := moves[layer]; ok {
			pass.Reportf(pos, "internal/%s isn't a LAY-6 layer: its code belongs in %s (LAY-6)", layer, to)
		} else {
			pass.Reportf(pos, "internal/%s isn't a LAY-6 layer: move it into foundation, config, domain, ports, jobs, core, repository, adapters, http or workers (LAY-6)", layer)
		}

		return false
	case flat[layer] && item != "" && !where.IsTestInfra(pkg.Rel, pkg.Name):
		pass.Reportf(pos, "internal/%s is flat: move %s into it, split by file (LAY-6)", layer, pkg.Rel)
	case perThing[layer] && item == "":
		if layer != "repository" {
			pass.Reportf(pos, "internal/%s holds one package per thing, and no code of its own: move this into one, or into cmd (LAY-6)", layer)
		}

		return false
	}

	return true
}

// reportConstructors reports constructors in ports, which holds contracts and
// their argument and result types only.
func reportConstructors(pass *analysis.Pass) {
	for _, file := range where.Files(pass) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && fn.Name.IsExported() && strings.HasPrefix(fn.Name.Name, "New") {
				pass.Reportf(fn.Name.Pos(), "ports holds contracts only: %s belongs with an implementation (LAY-6)", fn.Name.Name)
			}
		}
	}
}

func checkImport(pass *analysis.Pass, pkg where.Package, spec *ast.ImportSpec) {
	path, err := strconv.Unquote(spec.Path.Value)
	if err != nil {
		return
	}

	if rel, ok := pkg.Import(path); ok {
		checkInternalImport(pass, pkg, spec, rel, importedName(pass, spec))
		return
	}

	if isTestLibrary(path) && !where.IsTestInfra(pkg.Rel, pkg.Name) {
		pass.Reportf(spec.Pos(), "%s is test code: only _test.go files and other test packages import it (TEST-13)", path)
		return
	}

	checkSDKImport(pass, pkg, spec, path)
}

// checkInternalImport checks an import of a package in the same module.
func checkInternalImport(pass *analysis.Pass, pkg where.Package, spec *ast.ImportSpec, to, toName string) {
	fromTest := where.IsTestInfra(pkg.Rel, pkg.Name)

	if !fromTest && where.IsTestInfra(to, toName) {
		pass.Reportf(spec.Pos(), "%s is test code: only _test.go files and other test packages import it (TEST-13)", to)
		return
	}

	fromLayer, fromItem := where.Layer(pkg.Rel)
	toLayer, toItem := where.Layer(to)

	if fromTest || !where.Within(pkg.Rel, "internal") || !where.IsLayer(fromLayer) {
		return // cmd, pkg and test packages may import anything; a package in no layer is already reported
	}

	if !where.Within(to, "internal") && toLayer != "cmd" {
		return // pkg/ is a library like any other
	}

	if toLayer == "config" || (toLayer != "cmd" && !where.IsLayer(toLayer)) {
		return // depguard keeps config in cmd; a package in no layer is reported at its own clause
	}

	if fromLayer == toLayer && fromItem != "" && fromItem == toItem {
		return // one component's own subpackages
	}

	if fromLayer == "repository" && toLayer == "adapters" {
		if !strings.HasSuffix(toItem, "store") {
			pass.Reportf(spec.Pos(), "a repository imports only its database's store adapter, internal/adapters/<name>store, not %s (LAY-7)", to)
		}

		return
	}

	for _, allowed := range mayImport[fromLayer] {
		if allowed == toLayer {
			return
		}
	}

	pass.Reportf(spec.Pos(), "%s may not import %s: %s imports only %s (LAY-7)",
		pkg.Rel, to, fromLayer, list(mayImport[fromLayer]))
}

// checkSDKImport reports an SDK imported by an adapter or repository that isn't
// its home. depguard reports it anywhere else.
func checkSDKImport(pass *analysis.Pass, pkg where.Package, spec *ast.ImportSpec, path string) {
	layer, item := where.Layer(pkg.Rel)
	if (layer != "adapters" && layer != "repository") || where.IsTestInfra(pkg.Rel, pkg.Name) {
		return
	}

	sdk, ok := where.SDKOf(path)
	if !ok || len(sdk.Homes) == 0 {
		return
	}

	if layer == "repository" && sdk.Driver {
		return
	}

	for _, home := range sdk.Homes {
		if layer == "adapters" && item == home {
			return
		}
	}

	homes := make([]string, len(sdk.Homes))
	for i, home := range sdk.Homes {
		homes[i] = "internal/adapters/" + home
	}

	pass.Reportf(spec.Pos(), "%s belongs in %s: each SDK has one home (LAY-7)", path, strings.Join(homes, " or "))
}

// isTestLibrary says whether path is the testing package or testify. depguard
// already bans testify's suite and mock everywhere.
func isTestLibrary(path string) bool {
	switch {
	case path == "testing" || strings.HasPrefix(path, "testing/"):
		return true
	case strings.HasPrefix(path, "github.com/stretchr/testify/suite"), strings.HasPrefix(path, "github.com/stretchr/testify/mock"):
		return false
	default:
		return path == "github.com/stretchr/testify" || strings.HasPrefix(path, "github.com/stretchr/testify/")
	}
}

func importedName(pass *analysis.Pass, spec *ast.ImportSpec) string {
	if name := pass.TypesInfo.PkgNameOf(spec); name != nil {
		return name.Imported().Name()
	}

	return ""
}

// list writes layers as "a, b and c".
func list(layers []string) string {
	if len(layers) == 1 {
		return layers[0]
	}

	return strings.Join(layers[:len(layers)-1], ", ") + " and " + layers[len(layers)-1]
}

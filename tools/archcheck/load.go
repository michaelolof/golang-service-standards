package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/mod/modfile"
	"golang.org/x/tools/go/packages"
)

// module is the project, loaded once for every check.
type module struct {
	root string
	path string
	// code holds each package as it's built, without tests.
	code []*packages.Package
	// all adds every test variant, so a fake in a test counts as an implementation.
	all []*packages.Package
	// loaded adds the generated main of each test binary, where the call graph
	// of the test code starts.
	loaded []*packages.Package

	ignoredOnce sync.Once
	ignored     []ignoredFile

	// ignoreDirs are go.mod's ignore directives: folders the go command leaves
	// out of ./..., such as ./node_modules.
	ignoreDirs []string
}

// ignoredFile is a Go file of the module that this machine's build leaves out,
// such as a _windows_test.go file on Linux, read for its syntax only.
type ignoredFile struct {
	fset *token.FileSet
	file *ast.File
	rel  string
}

// ignoredFiles returns the module's Go files that the build leaves out on this
// machine. The checks that read names, such as which tests exist, read them
// too, so a finding doesn't depend on the machine it runs on. One that can't
// be parsed is left out.
func (m *module) ignoredFiles() []ignoredFile {
	m.ignoredOnce.Do(func() {
		fset := token.NewFileSet()
		seen := map[string]bool{}

		for _, p := range m.all {
			for _, path := range p.IgnoredFiles {
				if seen[path] || !strings.HasSuffix(path, ".go") {
					continue
				}

				seen[path] = true

				file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
				if err == nil {
					m.ignored = append(m.ignored, ignoredFile{fset: fset, file: file, rel: m.rel(path)})
				}
			}
		}
	})

	return m.ignored
}

// testTags are the build tags of every kind of test in the module (TEST-1), as
// task test:build vets them. An e2e suite is its own module.
const testTags = "integration,flow,contract,perf"

// checks run in this order. Each returns its problems.
var checks = []func(*module) ([]problem, error){
	checkArchitectureFolder,
	checkReadmes,
	checkEnvExample,
	checkMigrations,
	checkTestHomes,
	checkFlowFiles,
	checkFlowSetup,
	checkPromises,
	checkUncalledTestCode,
	checkOpenAPI,
	checkHandlerTests,
	checkBinaries,
	checkInterfaces,
	checkAssertions,
	checkKinds,
}

// check loads the module at root and runs every check.
func check(root string) ([]problem, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	m, err := load(root)
	if err != nil {
		return nil, err
	}

	var problems []problem

	for _, c := range checks {
		found, err := c(m)
		if err != nil {
			return nil, err
		}

		problems = append(problems, found...)
	}

	sortProblems(problems)

	return problems, nil
}

// load reads the module path from go.mod and type-checks every package, with
// its tests and every kind's tag (TEST-1). It keeps the syntax of every
// dependency too, because the call graph of the test code goes through them,
// such as the testing package calling each test.
func load(root string) (*module, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	path := modfile.ModulePath(data)
	if path == "" {
		return nil, errors.New("go.mod has no module line")
	}

	var ignoreDirs []string

	if mf, err := modfile.ParseLax("go.mod", data, nil); err == nil {
		for _, ig := range mf.Ignore {
			ignoreDirs = append(ignoreDirs, ig.Path)
		}
	}

	const mode = packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports | packages.NeedDeps |
		packages.NeedTypes | packages.NeedTypesSizes | packages.NeedSyntax | packages.NeedTypesInfo

	cfg := &packages.Config{Dir: root, Mode: mode, Tests: true, BuildFlags: []string{"-tags=" + testTags}}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load the module: %w", err)
	}

	m := &module{root: root, path: path, loaded: pkgs, ignoreDirs: ignoreDirs}

	// A package whose dependency doesn't build is ill-typed too, though it
	// reports no error of its own, so every package in the graph is read.
	var broken []string

	seen := map[string]bool{}

	packages.Visit(pkgs, nil, func(p *packages.Package) {
		for _, e := range p.Errors {
			if msg := e.Error(); !seen[msg] {
				seen[msg] = true
				broken = append(broken, msg)
			}
		}
	})

	for _, p := range pkgs {
		if strings.HasSuffix(p.ID, ".test") {
			continue // the generated test main
		}

		m.all = append(m.all, p)

		if p.ID == p.PkgPath {
			m.code = append(m.code, p)
		}
	}

	if len(broken) > 0 {
		return nil, fmt.Errorf("the module doesn't build, so its checks can't run:\n%s", strings.Join(broken, "\n"))
	}

	return m, nil
}

// relPkg returns path relative to the module, and false when it's outside it.
func (m *module) relPkg(path string) (string, bool) {
	if path == m.path {
		return "", true
	}

	return strings.CutPrefix(path, m.path+"/")
}

// within says whether rel is dir or a package under it.
func within(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

// isTestPackage says whether a package of the module is test code: anything
// under internal/testutils or internal/testutil, or a package named …test or
// …tests (TEST-13). The layers analyzer reads it the same way.
func isTestPackage(rel, name string) bool {
	return within(rel, "internal/testutils") || within(rel, "internal/testutil") ||
		strings.HasSuffix(name, "test") || strings.HasSuffix(name, "tests")
}

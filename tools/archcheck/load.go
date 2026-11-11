package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
}

// checks run in this order. Each returns its problems.
var checks = []func(*module) ([]problem, error){
	checkArchitectureFolder,
	checkReadmes,
	checkEnvExample,
	checkMigrations,
	checkOpenAPI,
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
// its tests and the integration tag the lint config also builds with (TEST-1).
func load(root string) (*module, error) {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("read go.mod: %w", err)
	}

	path := modfile.ModulePath(data)
	if path == "" {
		return nil, errors.New("go.mod has no module line")
	}

	cfg := &packages.Config{
		Dir:        root,
		Mode:       packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedTypes | packages.NeedSyntax | packages.NeedTypesInfo,
		Tests:      true,
		BuildFlags: []string{"-tags=integration"},
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load the module: %w", err)
	}

	m := &module{root: root, path: path}

	var broken []string

	for _, p := range pkgs {
		for _, e := range p.Errors {
			broken = append(broken, e.Error())
		}

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

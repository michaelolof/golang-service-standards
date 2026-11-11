package main

import (
	"fmt"
	"strings"

	"golang.org/x/tools/go/packages"
)

// testLibraries are the test code a binary must not carry, outside the module.
var testLibraries = []string{
	"testing", "github.com/stretchr/testify", "github.com/golang/mock", "go.uber.org/mock",
	"github.com/testcontainers/testcontainers-go",
}

// checkBinaries reports test code that a library brings into a binary under
// cmd/ (TEST-13). Test code the module itself imports is reported at the import
// by the layers analyzer, which is where an exception can go, so this reports
// only what arrives through a package outside the module.
func checkBinaries(m *module) ([]problem, error) {
	cfg := &packages.Config{Dir: m.root, Mode: packages.NeedName | packages.NeedFiles | packages.NeedImports | packages.NeedDeps}

	pkgs, err := packages.Load(cfg, "./cmd/...")
	if err != nil {
		return nil, fmt.Errorf("load the binaries: %w", err)
	}

	var problems []problem

	for _, main := range pkgs {
		if main.Name != "main" || len(main.GoFiles) == 0 {
			continue
		}

		for _, edge := range testCode(m, main) {
			problems = append(problems, problem{
				path: m.rel(main.GoFiles[0]),
				msg:  fmt.Sprintf("this binary carries test code: %s imports %s. Replace that library, or the package of ours that imports it (TEST-13)", edge.from, edge.to),
			})
		}
	}

	return problems, nil
}

type edge struct{ from, to string }

// testCode returns each import, from a package outside the module, of test
// code that main depends on.
func testCode(m *module, main *packages.Package) []edge {
	var found []edge

	seen := map[string]bool{}

	var visit func(p *packages.Package)
	visit = func(p *packages.Package) {
		if seen[p.PkgPath] {
			return
		}

		seen[p.PkgPath] = true
		_, inModule := m.relPkg(p.PkgPath)

		for path, dep := range p.Imports {
			if isTestLibrary(path) {
				if !inModule {
					found = append(found, edge{from: p.PkgPath, to: path})
				}

				continue // the layers analyzer reports the module's own import
			}

			if rel, ok := m.relPkg(path); ok && isTestPackage(rel, dep.Name) {
				continue // so does it for the module's own test packages
			}

			visit(dep)
		}
	}

	visit(main)

	return found
}

func isTestLibrary(path string) bool {
	for _, lib := range testLibraries {
		if path == lib || strings.HasPrefix(path, lib+"/") {
			return true
		}
	}

	return false
}

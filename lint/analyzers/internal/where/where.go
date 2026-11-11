// Package where tells an analyzer where a package sits in a project laid out by
// the standard: its module, its LAY-6 layer, and whether it's test
// infrastructure. Nothing here is configured per project. The module comes from
// the package being analyzed, and the layer from the folder under internal/.
package where

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

// Layers are LAY-6's folders under internal/, plus testutils.
var Layers = []string{
	"foundation", "config", "domain", "ports", "jobs", "core",
	"repository", "adapters", "http", "workers", "testutils",
}

// Package is where one package sits in its module.
type Package struct {
	// Module is the module path.
	Module string
	// Rel is the package path relative to the module, such as
	// "internal/core/transfer". It's "" for the module's root package.
	Rel string
	// Name is the package name.
	Name string
}

// Of returns where pass's package sits. It returns false when the module can't
// be worked out, which happens only outside the cmd/, internal/ and pkg/
// folders of a driver that doesn't report the module.
func Of(pass *analysis.Pass) (Package, bool) {
	path := pass.Pkg.Path()
	module := moduleOf(pass)

	if module == "" {
		return Package{}, false
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(path, module), "/")
	if path != module && !strings.HasPrefix(path, module+"/") {
		return Package{}, false
	}

	return Package{Module: module, Rel: rel, Name: pass.Pkg.Name()}, true
}

// moduleOf returns the module path. golangci-lint reports it. Other drivers may
// not, so the fallback cuts the package path before its first cmd, internal or
// pkg folder.
func moduleOf(pass *analysis.Pass) string {
	if pass.Module != nil && pass.Module.Path != "" {
		return pass.Module.Path
	}

	parts := strings.Split(pass.Pkg.Path(), "/")
	for i, part := range parts {
		if i > 0 && (part == "cmd" || part == "internal" || part == "pkg") {
			return strings.Join(parts[:i], "/")
		}
	}

	return ""
}

// Import returns path relative to the module, and false when path is outside it.
func (p Package) Import(path string) (string, bool) {
	if path == p.Module {
		return "", true
	}

	return strings.CutPrefix(path, p.Module+"/")
}

// Layer returns the LAY-6 layer and the folder under it, for a path relative to
// the module: "core" and "transfer" for internal/core/transfer/cancel. The layer
// is "cmd" or "pkg" for those folders, and the folder under internal/ when it
// isn't one of LAY-6's. It's "" outside cmd, internal and pkg.
func Layer(rel string) (layer, item string) {
	parts := strings.Split(rel, "/")

	switch {
	case parts[0] == "cmd" || parts[0] == "pkg":
		return parts[0], at(parts, 1)
	case parts[0] == "internal":
		return at(parts, 1), at(parts, 2)
	default:
		return "", ""
	}
}

func at(parts []string, i int) string {
	if i < len(parts) {
		return parts[i]
	}

	return ""
}

// Within says whether rel is dir or a package under it.
func Within(rel, dir string) bool {
	return rel == dir || strings.HasPrefix(rel, dir+"/")
}

// IsTestInfra says whether a package is test infrastructure: anything under
// internal/testutils, or internal/testutil, the name the lint config also
// excludes while test restructuring is paused (TEST-14); a test-helper package
// named <pkg>test (LAY-8, TEST-13); or a package that only holds tests, named
// …tests.
func IsTestInfra(rel, name string) bool {
	return Within(rel, "internal/testutils") || Within(rel, "internal/testutil") ||
		strings.HasSuffix(name, "test") || strings.HasSuffix(name, "tests")
}

// IsLayer says whether name is one of LAY-6's folders.
func IsLayer(name string) bool {
	return slices.Contains(Layers, name)
}

// Files returns pass's files that aren't tests, in the order golangci-lint
// gives them. The shared config doesn't lint tests (TEST-14), and other drivers
// may still pass them.
func Files(pass *analysis.Pass) []*ast.File {
	var files []*ast.File

	for _, file := range pass.Files {
		if !strings.HasSuffix(FileName(pass, file), "_test.go") {
			files = append(files, file)
		}
	}

	return files
}

// FileName returns the base name of file.
func FileName(pass *analysis.Pass, file *ast.File) string {
	return filepath.Base(pass.Fset.Position(file.Package).Filename)
}

// FileOf returns the file that holds pos, or nil.
func FileOf(pass *analysis.Pass, pos token.Pos) *ast.File {
	for _, file := range pass.Files {
		if file.FileStart <= pos && pos <= file.FileEnd {
			return file
		}
	}

	return nil
}

// PackageClause returns where to report something about the whole package: the
// package clause of the file named name if there is one, otherwise of the first
// file in name order. A //nolint on that line covers the finding.
func PackageClause(pass *analysis.Pass, name string) token.Pos {
	files := Files(pass)
	if len(files) == 0 {
		return token.NoPos
	}

	first := files[0]

	for _, file := range files {
		base := FileName(pass, file)
		if base == name {
			return file.Name.Pos()
		}

		if base < FileName(pass, first) {
			first = file
		}
	}

	return first.Name.Pos()
}

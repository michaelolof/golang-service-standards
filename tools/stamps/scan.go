package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// stampPackages are the import paths whose calls take a stamp.
var stampPackages = []string{"/errx", "/httperr", "/logger"}

// skippedDirs hold no service code, or code with stamps of its own. They count only
// at the project's root, because internal/http/platform/web is service code.
var skippedDirs = map[string]bool{".git": true, ".standards": true, "node_modules": true, "scripts": true, "vendor": true, "web": true}

type stampUse struct {
	value string
	place string
	test  bool
}

// skipped says whether a file, given relative to the project's root, is outside
// the code stamps are checked in.
func skipped(relative string) bool {
	first, _, _ := strings.Cut(filepath.ToSlash(relative), "/")

	return skippedDirs[first]
}

// goFiles lists the project's Go files.
func goFiles(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			if skippedDirs[entry.Name()] && filepath.Dir(path) == filepath.Clean(root) {
				return filepath.SkipDir
			}

			return nil
		}

		if strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", root, err)
	}

	return files, nil
}

// stampsIn returns every stamp in one file's source. A stamp is the first integer
// literal passed to a call on a stamp package, and 0 means "no stamp", which tests use.
func stampsIn(relative string, src []byte) ([]stampUse, error) {
	files := token.NewFileSet()

	parsed, err := parser.ParseFile(files, relative, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	names := stampPackageNames(parsed)
	test := strings.HasSuffix(relative, "_test.go")

	var uses []stampUse

	ast.Inspect(parsed, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}

		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		pkg, ok := selector.X.(*ast.Ident)
		if !ok || !names[pkg.Name] {
			return true
		}

		for _, arg := range call.Args {
			literal, ok := arg.(*ast.BasicLit)
			if !ok || literal.Kind != token.INT {
				continue
			}

			if literal.Value != "0" {
				place := fmt.Sprintf("%s:%d", filepath.ToSlash(relative), files.Position(literal.Pos()).Line)
				uses = append(uses, stampUse{value: literal.Value, place: place, test: test})
			}

			break
		}

		return true
	})

	return uses, nil
}

// stampPackageNames is what this file calls the stamp packages, so an alias counts.
func stampPackageNames(file *ast.File) map[string]bool {
	names := map[string]bool{}

	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}

		for _, suffix := range stampPackages {
			if !strings.HasSuffix(path, suffix) {
				continue
			}

			name := path[strings.LastIndex(path, "/")+1:]
			if spec.Name != nil {
				name = spec.Name.Name
			}

			names[name] = true
		}
	}

	return names
}

// allStamps returns every stamp in the project's working tree, in a stable order.
func allStamps(root string) ([]stampUse, error) {
	files, err := goFiles(root)
	if err != nil {
		return nil, err
	}

	var uses []stampUse

	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		relative, _ := filepath.Rel(root, path)

		found, err := stampsIn(relative, src)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relative, err)
		}

		uses = append(uses, found...)
	}

	sort.Slice(uses, func(i, j int) bool { return uses[i].place < uses[j].place })

	return uses, nil
}

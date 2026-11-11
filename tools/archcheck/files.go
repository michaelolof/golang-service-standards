package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// checkArchitectureFolder fails while internal/architecture exists: its tests
// repeat checks the standard now ships, so a project would keep two versions of
// the same rule (TEST-13).
func checkArchitectureFolder(m *module) ([]problem, error) {
	info, err := os.Stat(filepath.Join(m.root, "internal", "architecture"))
	if err != nil || !info.IsDir() {
		return nil, nil
	}

	return []problem{{path: "internal/architecture", msg: "the standard's analyzers and tools/archcheck now check these rules: delete internal/architecture and the arch task (TEST-13)"}}, nil
}

// checkReadmes reports a README.md beside Go code under internal/ or cmd/
// (LAY-11). A folder with no Go files, such as testdata, may have one.
func checkReadmes(m *module) ([]problem, error) {
	var problems []problem

	for _, top := range []string{"internal", "cmd"} {
		dir := filepath.Join(m.root, top)
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || !d.IsDir() {
				return err
			}

			entries, err := os.ReadDir(path)
			if err != nil {
				return err
			}

			readme, hasGo := "", false

			for _, e := range entries {
				switch {
				case strings.EqualFold(e.Name(), "README.md"):
					readme = e.Name()
				case !e.IsDir() && strings.HasSuffix(e.Name(), ".go"):
					hasGo = true
				}
			}

			if readme != "" && hasGo {
				problems = append(problems, problem{
					path: m.rel(filepath.Join(path, readme)),
					msg:  "a README.md beside Go code goes stale: move what it says into the package's doc comment (LAY-11)",
				})
			}

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", top, err)
		}
	}

	return problems, nil
}

// envLine matches a variable in .env.example, set or commented out:
// NAME=value, # NAME=value, export NAME=value.
var envLine = regexp.MustCompile(`^\s*(?:#\s*)?(?:export\s+)?([A-Z][A-Z0-9_]*)\s*=`)

// checkEnvExample checks that .env.example lists every variable an env tag in
// internal/config or cmd/ reads (LAY-13). The other direction is for review:
// the file also lists variables for docker compose and for SDKs that read the
// environment themselves, which no tag reads, and a line in it has nowhere to
// record that.
func checkEnvExample(m *module) ([]problem, error) {
	read := map[string]problem{}

	for _, p := range m.code {
		rel, _ := m.relPkg(p.PkgPath)
		if !within(rel, "internal/config") && !within(rel, "cmd") {
			continue
		}

		for _, file := range p.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				field, ok := n.(*ast.Field)
				if !ok || field.Tag == nil {
					return true
				}

				tag, err := strconv.Unquote(field.Tag.Value)
				if err != nil {
					return true
				}

				value, ok := reflect.StructTag(tag).Lookup("env")
				if name, _, _ := strings.Cut(value, ","); ok && name != "" {
					if _, seen := read[name]; !seen {
						read[name] = m.at(p.Fset, field.Tag.Pos(), "%s is read here, and .env.example doesn't list it (LAY-13)", name)
					}
				}

				return true
			})
		}
	}

	listed, err := readEnvExample(filepath.Join(m.root, ".env.example"))
	if os.IsNotExist(err) {
		if len(read) == 0 {
			return nil, nil
		}

		return []problem{{path: ".env.example", msg: "missing: list every variable the config's env tags read (LAY-13)"}}, nil
	}

	if err != nil {
		return nil, err
	}

	if len(read) == 0 {
		return []problem{{path: ".env.example", msg: "nothing in internal/config or cmd/ has an env tag: each config field declares its variable in one, so this file can be checked (LAY-13)"}}, nil
	}

	var problems []problem

	for name, p := range read {
		if _, ok := listed[name]; !ok {
			problems = append(problems, p)
		}
	}

	return problems, nil
}

// readEnvExample returns each variable .env.example lists, with its line.
func readEnvExample(path string) (map[string]int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	listed := map[string]int{}
	scanner := bufio.NewScanner(f)

	for line := 1; scanner.Scan(); line++ {
		if match := envLine.FindStringSubmatch(scanner.Text()); match != nil {
			if _, seen := listed[match[1]]; !seen {
				listed[match[1]] = line
			}
		}
	}

	return listed, scanner.Err()
}

var migrationName = regexp.MustCompile(`^([0-9]{14})_[a-z0-9_]+\.up\.(sql|cql|json)$`)

// checkMigrations reports a file in db/migrations whose name isn't
// YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, or whose timestamp another file has
// (DATA-11). Go files, such as the embed shim, and dotfiles are skipped.
func checkMigrations(m *module) ([]problem, error) {
	entries, err := os.ReadDir(filepath.Join(m.root, "db", "migrations"))
	if os.IsNotExist(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	var problems []problem

	seen := map[string]string{}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasSuffix(name, ".go") || strings.HasPrefix(name, ".") || strings.EqualFold(name, "README.md") {
			continue
		}

		path := "db/migrations/" + name

		match := migrationName.FindStringSubmatch(name)
		if match == nil {
			problems = append(problems, problem{path: path, msg: "a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)"})
			continue
		}

		if other, ok := seen[match[1]]; ok {
			problems = append(problems, problem{path: path, msg: "shares its timestamp with " + other + ": each migration has its own (DATA-11)"})
		}

		seen[match[1]] = name
	}

	return slices.Clip(problems), nil
}

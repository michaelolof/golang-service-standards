// Package repoverbs checks the names of a repository's methods (DATA-3).
//
//   - The first word of an exported Repo method isn't a banned verb: Insert,
//     Fetch, Lookup, Load, Read, Save, Put or Search. The closed set is Get,
//     Find, List, Count, Create, CreateOrGet, Update, Upsert and Delete, plus
//     domain transition verbs such as Cancel, Settle and MarkX, which no list
//     can hold, so only the banned verbs are reported.
//   - The name doesn't repeat the package's name: transfers.Repo.Get, not
//     GetTransfer.
package repoverbs

import (
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/repo"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks repository method names.
var Analyzer = &analysis.Analyzer{
	Name: "repoverbs",
	Doc:  "checks that repository methods use the standard's verbs and don't repeat the package name",
	Run:  run,
}

var banned = map[string]string{
	"Insert": "Create", "Fetch": "Get or Find", "Lookup": "Get or Find", "Load": "Get or List",
	"Read": "Get or List", "Save": "Create, Update or Upsert", "Put": "Upsert", "Search": "List",
}

func run(pass *analysis.Pass) (any, error) {
	pkg, ok := where.Of(pass)
	if !ok || !repo.IsAggregate(pkg) {
		return nil, nil
	}

	named := repo.Type(pass)
	if named == nil {
		return nil, nil
	}

	for _, method := range repo.Methods(named) {
		name := method.Name()

		if instead, ok := banned[repo.Verb(name)]; ok {
			pass.Reportf(method.Pos(), "%s isn't a repository verb: use %s (DATA-3)", repo.Verb(name), instead)
		}

		if noun := repeats(name, pkg.Name); noun != "" {
			pass.Reportf(method.Pos(), "Repo.%s repeats the package's name, %q: %s.Repo already says it (DATA-3)", name, noun, pkg.Name)
		}
	}

	return nil, nil
}

// repeats returns the form of the package name that name contains, or "": the
// plural transfers, or the singular transfer. Forms shorter than four letters
// aren't checked, because they turn up inside other words.
func repeats(name, pkgName string) string {
	lower := strings.ToLower(name)

	for _, form := range forms(pkgName) {
		if len(form) >= 4 && strings.Contains(lower, form) {
			return form
		}
	}

	return ""
}

func forms(plural string) []string {
	switch {
	case strings.HasSuffix(plural, "ies"):
		return []string{plural, strings.TrimSuffix(plural, "ies") + "y"}
	case strings.HasSuffix(plural, "ses"), strings.HasSuffix(plural, "xes"):
		return []string{plural, strings.TrimSuffix(plural, "es")}
	case strings.HasSuffix(plural, "s"):
		return []string{plural, strings.TrimSuffix(plural, "s")}
	default:
		return []string{plural}
	}
}

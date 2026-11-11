// Package analyzers registers the standard's own analyzers with golangci-lint.
// Each is its own linter, so a justified exception names the one it silences:
//
//	import "example.com/svc/internal/adapters/scylla" //nolint:layers // the store adapter keeps its name until the rename (LAY-8)
package analyzers

import (
	"github.com/golangci/plugin-module-register/register"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/adapters"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/cmdshape"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/comments"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/constquery"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/ctxkeys"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/enums"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/errkinds"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/errmap"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/generics"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/gofihandlers"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/goroutines"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/jobs"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/layers"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/metricnames"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/names"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/repos"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/repoverbs"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/services"
	"golang.org/x/tools/go/analysis"
)

// All are the analyzers, in the order the README lists them.
var All = []*analysis.Analyzer{
	layers.Analyzer,
	cmdshape.Analyzer,
	names.Analyzer,
	enums.Analyzer,
	services.Analyzer,
	adapters.Analyzer,
	generics.Analyzer,
	errkinds.Analyzer,
	gofihandlers.Analyzer,
	errmap.Analyzer,
	jobs.Analyzer,
	ctxkeys.Analyzer,
	goroutines.Analyzer,
	repos.Analyzer,
	repoverbs.Analyzer,
	constquery.Analyzer,
	comments.Analyzer,
	metricnames.Analyzer,
}

func init() {
	for _, a := range All {
		register.Plugin(a.Name, func(any) (register.LinterPlugin, error) {
			return plugin{analyzer: a}, nil
		})
	}
}

// plugin adapts one analyzer to golangci-lint's module plugin system. The
// analyzers take no settings: everything they need comes from the package they
// read.
type plugin struct {
	analyzer *analysis.Analyzer
}

func (p plugin) BuildAnalyzers() ([]*analysis.Analyzer, error) {
	return []*analysis.Analyzer{p.analyzer}, nil
}

func (plugin) GetLoadMode() string { return register.LoadModeTypesInfo }

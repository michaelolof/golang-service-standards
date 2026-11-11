// Package metricnames checks the instruments a service creates from an
// OpenTelemetry metric.Meter (OBS-2).
//
//   - The name is a constant, <svc>.<capability>.<noun>, lowercase and dotted.
//   - It has no unit suffix: no _total, _ms or _seconds. The exporter adds them.
//   - The unit is set with metric.WithUnit.
//   - A duration, named ….duration, is a Float64Histogram in seconds, "s".
//
// Which attributes an instrument records is for review.
package metricnames

import (
	"go/ast"
	"go/constant"
	"regexp"
	"strings"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Analyzer checks metric instrument names and units.
var Analyzer = &analysis.Analyzer{
	Name:     "metricnames",
	Doc:      "checks that metric instruments are named <svc>.<capability>.<noun> with a unit set by metric.WithUnit",
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

const metricPkg = "go.opentelemetry.io/otel/metric"

var (
	instrument  = regexp.MustCompile(`^(Int64|Float64)(Observable)?(Counter|UpDownCounter|Histogram|Gauge)$`)
	dottedName  = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*){2,}$`)
	unitSuffix  = regexp.MustCompile(`_(total|ms|seconds)$`)
	durationEnd = ".duration"
)

func run(pass *analysis.Pass) (any, error) {
	insp, _ := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)

	insp.Preorder([]ast.Node{(*ast.CallExpr)(nil)}, func(n ast.Node) {
		call, _ := n.(*ast.CallExpr)

		fn := where.Callee(pass, call)
		if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != metricPkg || fn.Signature().Recv() == nil ||
			!instrument.MatchString(fn.Name()) || len(call.Args) == 0 {
			return
		}

		value := pass.TypesInfo.Types[call.Args[0]].Value
		if value == nil || value.Kind() != constant.String {
			pass.Reportf(call.Args[0].Pos(), "an instrument's name is a constant, <svc>.<capability>.<noun> (OBS-2)")
			return
		}

		name := constant.StringVal(value)

		switch {
		case !dottedName.MatchString(name):
			pass.Reportf(call.Args[0].Pos(), "an instrument is named <svc>.<capability>.<noun>, lowercase and dotted: %q (OBS-2)", name)
		case unitSuffix.MatchString(name):
			pass.Reportf(call.Args[0].Pos(), "%q has a unit suffix: set the unit with metric.WithUnit, and the exporter adds the suffix (OBS-2)", name)
		}

		unit, hasUnit := unitOf(pass, call.Args[1:])
		if !hasUnit {
			pass.Reportf(call.Pos(), "set %q's unit with metric.WithUnit, in UCUM: s, By, {job} (OBS-2)", name)
		}

		if strings.HasSuffix(name, durationEnd) && (fn.Name() != "Float64Histogram" || (unit != "" && unit != "s")) {
			pass.Reportf(call.Pos(), "a duration is a Float64Histogram measured in seconds, with metric.WithUnit(\"s\") (OBS-2)")
		}
	})

	return nil, nil
}

// unitOf returns the constant unit passed with metric.WithUnit among opts, and
// whether WithUnit was passed at all.
func unitOf(pass *analysis.Pass, opts []ast.Expr) (string, bool) {
	for _, opt := range opts {
		call, ok := ast.Unparen(opt).(*ast.CallExpr)
		if !ok || !where.Calls(pass, call, metricPkg, "WithUnit") || len(call.Args) != 1 {
			continue
		}

		if value := pass.TypesInfo.Types[call.Args[0]].Value; value != nil && value.Kind() == constant.String {
			return constant.StringVal(value), true
		}

		return "", true
	}

	return "", false
}

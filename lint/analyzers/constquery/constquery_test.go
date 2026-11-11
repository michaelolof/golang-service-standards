package constquery_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/constquery"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestConstquery(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), constquery.Analyzer, "example.com/constquery/...")
}

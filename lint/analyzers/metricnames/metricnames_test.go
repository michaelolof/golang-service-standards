package metricnames_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/metricnames"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestMetricnames(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), metricnames.Analyzer, "example.com/metricnames/...")
}

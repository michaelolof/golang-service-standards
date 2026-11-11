package adapters_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/adapters"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAdapters(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), adapters.Analyzer, "example.com/adapters/...")
}

package goroutines_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/goroutines"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestGoroutines(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), goroutines.Analyzer, "example.com/goroutines/...")
}

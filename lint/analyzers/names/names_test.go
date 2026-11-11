package names_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/names"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestNames(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), names.Analyzer, "example.com/names/...")
}

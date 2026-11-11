package generics_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/generics"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestGenerics(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), generics.Analyzer, "example.com/generics/...")
}

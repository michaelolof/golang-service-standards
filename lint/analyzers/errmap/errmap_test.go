package errmap_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/errmap"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestErrmap(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), errmap.Analyzer, "example.com/errmap/...")
}

package repoverbs_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/repoverbs"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRepoverbs(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), repoverbs.Analyzer, "example.com/repoverbs/...")
}

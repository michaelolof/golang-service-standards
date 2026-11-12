package txshape_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/txshape"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTxshape(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), txshape.Analyzer, "example.com/txshape/...")
}

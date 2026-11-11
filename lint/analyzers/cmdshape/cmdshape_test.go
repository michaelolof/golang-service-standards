package cmdshape_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/cmdshape"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCmdshape(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), cmdshape.Analyzer, "example.com/cmdshape/...")
}

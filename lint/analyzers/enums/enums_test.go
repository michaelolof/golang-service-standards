package enums_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/enums"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestEnums(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), enums.Analyzer, "example.com/enums/...")
}

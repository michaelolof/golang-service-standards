package errkinds_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/errkinds"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestErrkinds(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), errkinds.Analyzer, "example.com/errkinds/...")
}

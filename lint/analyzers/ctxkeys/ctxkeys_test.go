package ctxkeys_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/ctxkeys"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestCtxkeys(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), ctxkeys.Analyzer, "example.com/ctxkeys/...")
}

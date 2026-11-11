package gofihandlers_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/gofihandlers"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestGofihandlers(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), gofihandlers.Analyzer, "example.com/gofihandlers/...")
}

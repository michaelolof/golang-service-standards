package services_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/services"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestServices(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), services.Analyzer, "example.com/services/...")
}

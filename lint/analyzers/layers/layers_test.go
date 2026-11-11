package layers_test

import (
	"path/filepath"
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/layers"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestLayers(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), layers.Analyzer, "example.com/layers/...")
}

func TestLayersInAModule(t *testing.T) {
	analysistest.Run(t, filepath.Join(fixture.Dir(), "modules", "layout"), layers.Analyzer, "./...")
}

package repos_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/repos"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRepos(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), repos.Analyzer, "example.com/repos/...")
}

package comments_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/comments"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestComments(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), comments.Analyzer, "example.com/comments/...")
}

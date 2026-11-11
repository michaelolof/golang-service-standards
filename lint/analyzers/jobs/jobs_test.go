package jobs_test

import (
	"testing"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/fixture"
	"github.com/michaelolof/golang-service-standards/lint/analyzers/jobs"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestJobs(t *testing.T) {
	analysistest.Run(t, fixture.Dir(), jobs.Analyzer, "example.com/jobs/...")
}

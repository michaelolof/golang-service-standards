package requiredtag_test

import (
	"testing"

	"github.com/michaelolof/requiredtag"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestRequiredTag(t *testing.T) {
	a, err := requiredtag.New(requiredtag.Settings{
		Tags:              []string{"validate:required", "lint:required"},
		AllowEmptyReturns: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, analysistest.TestData(), a, "a", "b")
}

func TestEmptyReturnsAreReportedByDefault(t *testing.T) {
	a, err := requiredtag.New(requiredtag.Settings{Tags: []string{"lint:required"}})
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, analysistest.TestData(), a, "c")
}

func TestSettingsAreChecked(t *testing.T) {
	for _, tags := range [][]string{nil, {"required"}, {":required"}, {"lint:"}} {
		if _, err := requiredtag.New(requiredtag.Settings{Tags: tags}); err == nil {
			t.Errorf("New(Tags: %q) returned no error", tags)
		}
	}
}

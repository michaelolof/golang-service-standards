package wrapgaps_test

import (
	"testing"

	"github.com/michaelolof/wrapgaps"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestWrapGaps(t *testing.T) {
	a, err := wrapgaps.New(wrapgaps.Settings{
		ExtraIgnoreSigs:    []string{"gofi.Context).Next(", "gofi.Context).Send"},
		IgnorePackageGlobs: []string{"*/ignored", "ignored"},
	})
	if err != nil {
		t.Fatal(err)
	}

	analysistest.Run(t, analysistest.TestData(), a, "a")
}

func TestSettingsAreChecked(t *testing.T) {
	if _, err := wrapgaps.New(wrapgaps.Settings{IgnorePackageGlobs: []string{"["}}); err == nil {
		t.Error("New with a bad glob returned no error")
	}
}

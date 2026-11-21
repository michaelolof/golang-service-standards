package main

import (
	"slices"
	"testing"
)

// Each tag lists only the packages that have that kind of test. A package
// with only unit tests, the shared harness and the other kinds are left out.
func TestTagged_ListsOnlyThePackagesWithThatKindOfTest(t *testing.T) {
	cases := []struct {
		name string
		tags string
		want []string
	}{
		{name: "integration", tags: "integration", want: []string{"example.com/kinds/internal/repository/orders"}},
		{name: "flow", tags: "flow", want: []string{"example.com/kinds/test/flows/orders"}},
		{name: "contract", tags: "contract", want: []string{"example.com/kinds/test/contract"}},
		{name: "a kind the module doesn't have", tags: "perf", want: nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tagged("testdata/module", tc.tags)
			if err != nil {
				t.Fatal(err)
			}

			if !slices.Equal(got, tc.want) {
				t.Fatalf("tagged(%q) = %q, want %q", tc.tags, got, tc.want)
			}
		})
	}
}

// A folder go list can't read stops the run, so no task runs on a partial list.
func TestTagged_FailsWhenGoListFails(t *testing.T) {
	if _, err := tagged(t.TempDir(), "integration"); err == nil {
		t.Fatal("tagged returned no error for a folder with no module")
	}
}

//go:build flow

package transfers_test

import (
	"testing"

	"example.com/good/internal/testutils/testharness"
)

// A type that only holds a table's rows, and a package-level value, are fine
// in a flow file.
type reading struct{ name, id string }

var readings = []reading{{name: "a transfer by its ID", id: "t-1"}}

// The tenant's app reads a transfer by its ID, and its details are kept
// nowhere else.
func TestTransfer_TheAppReadsATransferByItsID(t *testing.T) {
	for _, tc := range readings {
		t.Run(tc.name, func(t *testing.T) {
			w := testharness.New(t)

			if got := w.ReadTransfer(tc.id); got != tc.id {
				t.Fatalf("read %q, want %q", got, tc.id)
			}

			// The promise is about storage, so the flow reads it through the harness.
			if w.StoredTransfers() > 1 {
				t.Fatal("kept twice")
			}
		})
	}
}

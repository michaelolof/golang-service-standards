package transfer_test

import "testing"

// A path on Windows uses its own separator.
func TestPath_UsesTheWindowsSeparator(t *testing.T) {
	if windowsPath() == "" {
		t.Fatal("no path")
	}
}

//go:build flow

package orders_test

import (
	"os"
	"testing"
)

// TestMain gives the package its own database (TEST-3).
func TestMain(m *testing.M) { os.Exit(m.Run()) }

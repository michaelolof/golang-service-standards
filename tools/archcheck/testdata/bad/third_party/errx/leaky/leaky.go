// Package leaky is a library that imports testing outside a test.
package leaky

import "testing"

func Enabled() bool { return testing.Testing() }

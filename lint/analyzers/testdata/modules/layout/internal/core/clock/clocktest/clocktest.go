// A test package may import testing.
package clocktest

import "testing"

func New(t testing.TB) { t.Helper() }

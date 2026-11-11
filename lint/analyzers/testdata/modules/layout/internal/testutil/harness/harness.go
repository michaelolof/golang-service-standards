// The legacy name for test helpers is test code too. Moving it to testutils is
// paused with the tests, and the lint config excludes the folder until then.
package harness // want `internal/testutil isn't a LAY-6 layer: its code belongs in internal/testutils \(LAY-6\)`

import "testing"

func Start(t testing.TB) { t.Helper() }

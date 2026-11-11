package clock

import "example.com/layout/internal/testutil/harness" // want `internal/testutil/harness is test code: only _test.go files and other test packages import it \(TEST-13\)`

var _ = harness.Start

package workers // want `internal/workers holds one package per thing, and no code of its own: move this into one, or into cmd \(LAY-6\)`

import "example.com/layers/internal/workers/erasure"

var _ = erasure.Name

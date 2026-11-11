package exports

import (
	"example.com/layers/internal/jobs"
	"example.com/layers/internal/workers/erasure" // want `internal/workers/exports may not import internal/workers/erasure`
)

var (
	_ jobs.Job
	_ = erasure.Name
)

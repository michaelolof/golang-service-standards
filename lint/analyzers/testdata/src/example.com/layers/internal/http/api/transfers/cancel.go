package transfers

import (
	"example.com/layers/internal/adapters/kafka" // want `internal/http/api/transfers may not import internal/adapters/kafka: http imports only core, repository, domain, ports, foundation and http \(LAY-7\)`
	"example.com/layers/internal/core/transfer"
	"example.com/layers/internal/http/httperr"
	"example.com/layers/internal/jobs" // want `may not import internal/jobs`
	repo "example.com/layers/internal/repository/transfers"
)

var (
	_ = kafka.Name
	_ = transfer.Name
	_ = httperr.Name
	_ jobs.Job
	_ = repo.Name
)

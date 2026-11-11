package transfer

import (
	"example.com/layers/internal/config"
	"example.com/layers/internal/core/other"
	"example.com/layers/internal/domain"
	"example.com/layers/internal/domain/domaintest" // want `internal/domain/domaintest is test code: only _test.go files and other test packages import it \(TEST-13\)`
	"example.com/layers/internal/jobs"
	"example.com/layers/internal/ports"
	"example.com/layers/internal/repository/accounts" // want `internal/core/transfer may not import internal/repository/accounts: core imports only domain, ports, jobs, foundation and core \(LAY-7\)`
	"example.com/layers/internal/service/legacy"
	"example.com/layers/internal/testutils/harness" // want `internal/testutils/harness is test code`
	"example.com/layers/pkg/queue"
)

const Name = "transfer"

var (
	_ config.Mongo
	_ = other.Name
	_ domain.Transfer
	_ = domaintest.Transfer
	_ jobs.Job
	_ ports.Publisher
	_ = accounts.Name
	_ = legacy.Name
	_ = harness.Name
	_ = queue.Name
)

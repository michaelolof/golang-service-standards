package transfers

import (
	"example.com/layers/internal/adapters/crdbstore"
	"example.com/layers/internal/adapters/kafka" // want `a repository imports only its database's store adapter, internal/adapters/<name>store, not internal/adapters/kafka \(LAY-7\)`
	"example.com/layers/internal/core/other"
	"example.com/layers/internal/repository/accounts" // want `internal/repository/transfers may not import internal/repository/accounts: repository imports only domain, ports, jobs, foundation, core and adapters/<name>store \(LAY-7\)`
	"github.com/jackc/pgx/v5"
	"github.com/twmb/franz-go/pkg/kgo" // want `github.com/twmb/franz-go/pkg/kgo belongs in internal/adapters/kafka: each SDK has one home \(LAY-7\)`
)

const Name = "transfers"

var (
	_ = crdbstore.Name
	_ = kafka.Name
	_ = other.Name
	_ = accounts.Name
	_ pgx.Tx
	_ kgo.Client
)

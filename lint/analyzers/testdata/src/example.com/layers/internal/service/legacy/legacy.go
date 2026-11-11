package legacy // want `internal/service isn't a LAY-6 layer: its code belongs in internal/core \(LAY-6\)`

import "example.com/layers/internal/repository/transfers"

const Name = "legacy"

var _ = transfers.Name

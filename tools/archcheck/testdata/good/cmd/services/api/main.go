// Command api serves the API.
package main

import (
	"example.com/good/internal/adapters/queue"
	"example.com/good/internal/core/transfer"
	"example.com/good/internal/repository/transfers"
)

func main() { _, _ = transfer.New(transfer.Deps{Store: transfers.New(), Jobs: queue.New()}) }

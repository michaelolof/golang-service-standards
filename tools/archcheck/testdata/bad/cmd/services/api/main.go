// Command api serves the API.
package main

import (
	"example.com/bad/internal/adapters/mailer"
	"example.com/bad/internal/core/transfer"
	"example.com/bad/internal/foundation/clock"
	"example.com/bad/internal/repository/transfers"
	"github.com/michaelolof/errx/leaky"
)

func main() {
	_, _ = transfer.New(transfer.Deps{Store: transfers.New(), Clock: clock.System{}, Mailer: mailer.New()})
	_ = leaky.Enabled()
}

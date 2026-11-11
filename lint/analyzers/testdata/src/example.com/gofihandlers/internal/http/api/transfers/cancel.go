package transfers

import "github.com/michaelolof/gofi"

// CancelTransferHandler cancels a transfer.
var CancelTransferHandler = gofi.DefineHandler(gofi.RouteOptions{
	Info: gofi.Info{
		Tags:        []string{"transfers"},
		Description: "Cancels a transfer that has not reached the provider.",
	},

	Schema: &cancelTransferSchema{},

	Handler: func(c gofi.Context) error {
		_, err := gofi.ValidateAndBind[cancelTransferSchema](c)
		if err != nil {
			return err
		}

		return c.Send(200, nil)
	},
})

type cancelTransferSchema struct{}

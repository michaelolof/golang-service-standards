package transfers

import "github.com/michaelolof/gofi"

// listTransfers lists transfers.
var listTransfers = gofi.DefineHandler(gofi.RouteOptions{ // want `a handler is an exported variable named <Operation>Handler, such as CancelTransferHandler: rename listTransfers \(gofi HND-1, HND-2\)` `listTransfers's fields go in the order Info, Schema, Meta, Handler \(gofi HND-3\)` `listTransfers's schema is an unexported type named listTransfersSchema \(gofi HND-4\)` `listTransfers sets OperationId: leave it out, the method and path identify the route \(HTTP-5\)` `listTransfers's Info needs a Description written for API consumers \(gofi DOC-1, DOC-2\)` `listTransfers's Info has exactly one tag, the one its package is named after \(gofi DOC-1\)`
	Schema: &listSchema{},

	Info: gofi.Info{
		Tags:        []string{"transfers", "admin"},
		OperationId: "listTransfers",
	},

	Handler: func(c gofi.Context) error { return c.Send(200, c.Query("limit")) }, // want `a handler reads request data from its bound schema, not from c.Query \(gofi HND-8\)`
})

// GetTransferHandler is a second operation in the file.
var GetTransferHandler = gofi.DefineHandler(gofi.RouteOptions{ // want `one operation per file: move GetTransferHandler to a file named for it \(HTTP-2, gofi LAY-2\)` `the package is named after its OpenAPI tag: tag "payouts" goes in package payouts, not transfers \(HTTP-2, gofi LAY-3\)` `getTransferSchema is declared directly below GetTransferHandler, in the same file \(gofi HND-4\)`
	Info: gofi.Info{
		Tags:        []string{"payouts"},
		Description: "Gets a transfer.",
	},

	Schema: &getTransferSchema{},

	Handler: func(c gofi.Context) error { return c.Next() },
})

type listSchema struct{}

type getTransferSchema struct{}

var _ = listTransfers

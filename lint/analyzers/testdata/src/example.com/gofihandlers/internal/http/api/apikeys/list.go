package apikeys

import "github.com/michaelolof/gofi"

// ListAPIKeysHandler lists API keys: a tag of two words names the package apikeys.
var ListAPIKeysHandler = gofi.DefineHandler(gofi.RouteOptions{
	Info: gofi.Info{
		Tags:        []string{"api-keys"},
		Description: "Lists API keys.",
	},

	Schema: &listAPIKeysSchema{},

	Handler: func(c gofi.Context) error { return c.Send(200, nil) },
})

type listAPIKeysSchema struct{}

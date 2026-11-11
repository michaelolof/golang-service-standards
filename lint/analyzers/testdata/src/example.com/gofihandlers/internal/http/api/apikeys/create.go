package apikeys

import "github.com/michaelolof/gofi"

// CreateAPIKeyHandler creates an API key.
var CreateAPIKeyHandler = gofi.DefineHandler(gofi.RouteOptions{ // want `the tag "API_Keys" is lowercase kebab-case \(gofi DOC-1\)`
	Info: gofi.Info{
		Tags:        []string{"API_Keys"},
		Description: "Creates an API key.",
	},

	Schema: &createAPIKeySchema{},

	Handler: func(c gofi.Context) error { return c.Send(201, nil) },
})

type createAPIKeySchema struct{}

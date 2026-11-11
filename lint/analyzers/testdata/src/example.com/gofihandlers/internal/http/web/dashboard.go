package web

import "github.com/michaelolof/gofi"

// DashboardHandler serves files. A hidden route isn't documented, so it needs no tag.
var DashboardHandler = gofi.DefineHandler(gofi.RouteOptions{
	Info: gofi.Info{Hidden: true},

	Handler: func(c gofi.Context) error { return c.Next() },
})

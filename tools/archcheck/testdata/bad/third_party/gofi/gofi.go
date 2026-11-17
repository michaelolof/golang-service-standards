// Package gofi is a stand-in for the real library.
package gofi

// Context is a request's context.
type Context interface {
	Send(code int, obj any) error
}

// RouteOptions defines a route.
type RouteOptions struct {
	Schema  any
	Handler func(c Context) error
}

// DefineHandler defines a handler.
func DefineHandler(opts RouteOptions) RouteOptions { return opts }

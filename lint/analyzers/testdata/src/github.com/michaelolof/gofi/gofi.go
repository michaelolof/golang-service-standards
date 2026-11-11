package gofi

import "context"

type Context interface {
	Context() context.Context
	Send(code int, obj any) error
	Next() error
	Param(key string) string
	Query(key string) string
	HeaderVal(key string) string
}

type Info struct {
	Hidden      bool
	OperationId string
	Summary     string
	Description string
	Tags        []string
}

type RouteOptions struct {
	Info    Info
	Schema  any
	Meta    any
	Handler func(c Context) error
}

func DefineHandler(opts RouteOptions) RouteOptions { return opts }

func ValidateAndBind[T any](c Context) (*T, error) { return nil, nil }

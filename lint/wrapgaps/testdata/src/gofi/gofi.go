package gofi

import "errors"

type Context interface {
	Next() error
	Send(code int, obj any) error
	SendBytes(code int, b []byte) error
	SendString(code int, s string) error
	SendStream(code int, s any, sw func() error) error
	Close() error
}

func ValidateAndBind[T any](c Context) (*T, error) { return nil, errors.New("bind") }

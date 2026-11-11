package main

import "context"

type app struct{}

func wire(ctx context.Context, cfg apiConfig) (*app, func(), error) {
	_, _ = ctx, cfg
	helper()
	mapHTTP()

	return &app{}, func() {}, nil
}

func mapHTTP() {} // want `wire.go holds one function, wire: move mapHTTP out \(LAY-3\)`

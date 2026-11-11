package main

import "context"

type app struct{}

func wire(ctx context.Context, cfg okConfig) (*app, func(), error) {
	_, _ = ctx, cfg

	return &app{}, func() {}, nil
}

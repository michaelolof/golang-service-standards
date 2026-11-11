package main

type adminConfig struct{}

type app struct{}

func wire(cfg adminConfig) (*app, error) { return &app{}, nil } // want `wire's signature is func wire\(ctx context.Context, cfg <role>Config\) \(\*app, func\(\), error\) \(LAY-3\)`

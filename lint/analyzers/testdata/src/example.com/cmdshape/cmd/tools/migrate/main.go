// Command migrate is a tool, so it needs no wire.go.
package main

import "context"

func main() { _ = run(context.Background()) }

func run(context.Context) error { return nil }

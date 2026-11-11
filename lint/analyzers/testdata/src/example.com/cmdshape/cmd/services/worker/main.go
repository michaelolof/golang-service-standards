// Command worker has no wire.go.
package main // want `cmd/services/worker has no wire.go: one wire function builds every dependency at startup \(LAY-3\)`

import "context"

func main() { _ = run(context.Background()) }

func run(context.Context) error { return nil }

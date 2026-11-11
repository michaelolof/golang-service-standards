// Command admin has a wire with the wrong shape.
package main

import "context"

func main() { _ = run(context.Background()) }

func run(context.Context) error {
	_, err := wire(adminConfig{})
	return err
}

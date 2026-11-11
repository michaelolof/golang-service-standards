// Command ok follows LAY-2 and LAY-3.
package main

import "context"

func main() { _ = run(context.Background()) }

func run(ctx context.Context) error {
	_, cleanup, err := wire(ctx, okConfig{})
	if err != nil {
		return err
	}
	defer cleanup()

	return nil
}

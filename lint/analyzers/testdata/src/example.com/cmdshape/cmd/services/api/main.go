// Command api serves the API.
package main

import "context"

type apiConfig struct{} // want `main.go declares only main and run: move this type to the file named for it, such as config.go or wire.go \(LAY-2\)`

func main() { _ = run(context.Background()) }

func run(ctx context.Context) error {
	_, cleanup, err := wire(ctx, apiConfig{})
	if err == nil {
		cleanup()
	}

	return err
}

func helper() {} // want `main.go declares only main and run: move helper`

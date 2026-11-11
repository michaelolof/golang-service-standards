// Command archcheck runs the standard's checks that need the whole module, or
// files that aren't Go. The per-package checks are analyzers in lint/analyzers,
// run by task lint. This runs from task standards:check, the way tools/stamps does:
//
//	go run -C .standards/tools/archcheck . "$PWD"   // task archcheck
//
// It checks:
//
//   - that internal/architecture is gone: these checks replace it (TEST-13);
//   - no README.md beside Go code under internal/ or cmd/ (LAY-11);
//   - .env.example lists every variable the config's env tags read (LAY-13);
//   - migration file names (DATA-11);
//   - the committed OpenAPI documents in docs/openapi: a description for every
//     field and parameter, and an example for every scalar (HTTP-4);
//   - no binary in cmd/ carries test code that a library brings in (TEST-13);
//   - every interface has an implementation, every port a consumer, and no two
//     interfaces the same methods (ABS-1);
//   - every repository or adapter type that implements an interface asserts it (ABS-2);
//   - every error kind is created and checked somewhere (ERR-5).
//
// Its findings have no exceptions: each is fixed. A finding that's wrong is a
// bug in archcheck, and is fixed in the standard.
package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: archcheck <project root>")
		os.Exit(2)
	}

	problems, err := check(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "archcheck:", err)
		os.Exit(1)
	}

	for _, p := range problems {
		fmt.Fprintln(os.Stderr, p)
	}

	if len(problems) > 0 {
		fmt.Fprintf(os.Stderr, "archcheck: %d problems. Each is fixed: archcheck takes no exceptions.\n", len(problems))
		os.Exit(1)
	}
}

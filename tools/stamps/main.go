// Command stamps hands out the stamps that errx, httperr and logger calls take
// (ERR-1), and checks the ones that are new since the last push.
//
//	go run -C .standards/tools/stamps . fill "$PWD"   // task stamps
//	go run -C .standards/tools/stamps . check "$PWD"  // task stamps:check
//
// A developer writes __STAMP__ where a stamp belongs, and fill replaces each one
// with its own clock reading. fill records what it hands out in the git
// directory, so check can tell a stamp read from the clock from one that was typed.
package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// placeholder is what a developer writes where a stamp belongs. It's a valid Go
	// identifier, so a file holding one still parses.
	placeholder = "__STAMP__"
	lockTimeout = 30 * time.Second
	// staleLock is how old a lock must be before a crashed run is assumed to have left it.
	staleLock = 2 * time.Minute
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: stamps fill|check <project root>")
		os.Exit(2)
	}

	root, err := filepath.Abs(os.Args[2])
	if err == nil {
		err = run(os.Args[1], root)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(command, root string) error {
	switch command {
	case "fill":
		release, err := lock(root, lockTimeout)
		if err != nil {
			return err
		}
		defer release()

		filled, err := fill(root, realClock(), recordFile(root))
		if err != nil {
			return err
		}

		fmt.Printf("stamped %d\n", filled)

		return nil
	case "check":
		past, err := loadHistory(root)
		if err != nil {
			return err
		}

		problems, err := check(root, time.Now().UnixMilli(), past)
		if err != nil {
			return err
		}

		for _, problem := range problems {
			fmt.Fprintln(os.Stderr, problem)
		}

		if len(problems) > 0 {
			return errors.New("stamps: to change a stamp, replace it with " + placeholder + " and run task stamps (ERR-1)")
		}

		return nil
	default:
		return fmt.Errorf("unknown command %q", command)
	}
}

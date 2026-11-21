// Command testpackages prints the packages that have tests of one kind: those
// whose test files grow when that kind's build tag is set (TEST-1). task
// test:integration runs only these, so a package with only unit tests isn't
// rebuilt and run again under the tag:
//
//	go run -C .standards/tools/testpackages . -tags integration "$PWD"
//
// A build tag adds files to a package. It doesn't remove the untagged ones, so
// go test -tags integration ./... also builds and runs every unit test in every
// package. This compares go list with and without the tag, and lists each
// package that gains a test file. It prints them on one line, for a command
// line. It fails when no package has any, so a run never passes on nothing.
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
)

func main() {
	tags := flag.String("tags", "", "the build tag of one kind of test, such as integration")

	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: testpackages -tags <tag> <project root>")
	}

	flag.Parse()

	if *tags == "" || flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	pkgs, err := tagged(flag.Arg(0), *tags)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testpackages:", err)
		os.Exit(1)
	}

	if len(pkgs) == 0 {
		fmt.Fprintf(os.Stderr, "testpackages: no package has a test file with the %s tag, so there's nothing to run\n", *tags)
		os.Exit(1)
	}

	fmt.Println(strings.Join(pkgs, " "))
}

// tagged returns, sorted, the import path of each package of the module at
// root that has a test file only when tags are set.
func tagged(root, tags string) ([]string, error) {
	without, err := testFiles(root)
	if err != nil {
		return nil, err
	}

	with, err := testFiles(root, "-tags", tags)
	if err != nil {
		return nil, err
	}

	var found []string

	for pkg, files := range with {
		if slices.ContainsFunc(files, func(f string) bool { return !slices.Contains(without[pkg], f) }) {
			found = append(found, pkg)
		}
	}

	slices.Sort(found)

	return found, nil
}

// listFormat writes a package's import path, then each of its test files.
const listFormat = `{{.ImportPath}}{{range .TestGoFiles}} {{.}}{{end}}{{range .XTestGoFiles}} {{.}}{{end}}`

// testFiles returns the test files of each package of the module at root, as
// go list sees them with flags.
func testFiles(root string, flags ...string) (map[string][]string, error) {
	args := slices.Concat([]string{"list"}, flags, []string{"-f", listFormat, "./..."})

	cmd := exec.Command("go", args...)
	cmd.Dir = root

	var stderr bytes.Buffer

	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}

	files := map[string][]string{}

	for line := range strings.Lines(string(out)) {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}

		files[fields[0]] = fields[1:]
	}

	if len(files) == 0 {
		return nil, errors.New("go list found no packages in " + root)
	}

	return files, nil
}

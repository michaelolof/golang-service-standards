// Command genrules writes RULES.md: one table of every rule, built from the
// "Rules at a glance" table in each numbered document.
//
//	go run ./tools/genrules         // writes RULES.md
//	go run ./tools/genrules -check  // fails if RULES.md is out of date
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const output = "RULES.md"

// ruleRow matches a table row whose first cell is a rule ID, and captures the
// ID and the rule. A row may have a last column saying how the rule is enforced.
var ruleRow = regexp.MustCompile(`^\| ([A-Z]{3,4}-[0-9]+) \| (.*?)(?: \| [^|]*)? \|$`)

func main() {
	check := flag.Bool("check", false, "fail if RULES.md is out of date")
	flag.Parse()

	content, err := build()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *check {
		current, err := os.ReadFile(output)
		if err != nil || !bytes.Equal(current, content) {
			fmt.Fprintln(os.Stderr, "RULES.md is out of date: run go run ./tools/genrules")
			os.Exit(1)
		}

		return
	}

	if err := os.WriteFile(output, content, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func build() ([]byte, error) {
	files, err := filepath.Glob("[0-9][0-9]-*.md")
	if err != nil {
		return nil, err
	}

	sort.Strings(files)

	var out bytes.Buffer

	out.WriteString("# Every rule\n\n")
	out.WriteString("Generated from the \"Rules at a glance\" tables. Don't edit it by hand. Each document has the detail, the reasons and examples.\n")

	for _, file := range files {
		section, err := rulesIn(file)
		if err != nil {
			return nil, err
		}

		out.WriteString(section)
	}

	return out.Bytes(), nil
}

func rulesIn(file string) (string, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	title := strings.TrimPrefix(lines[0], "# ")

	var rows []string

	for _, line := range lines {
		if m := ruleRow.FindStringSubmatch(line); m != nil {
			rows = append(rows, fmt.Sprintf("| %s | %s |", m[1], m[2]))
		}
	}

	if len(rows) == 0 {
		return "", nil
	}

	return fmt.Sprintf("\n## [%s](%s)\n\n| ID | Rule |\n|---|---|\n%s\n", title, file, strings.Join(rows, "\n")), nil
}

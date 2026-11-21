package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
)

const promisesDir = "docs/promises"

// notReachable is the heading of a promise file's section for the promises no
// caller can reach (TEST-18).
const notReachable = "Not reachable from outside"

var (
	namedTest   = regexp.MustCompile("`(Test[A-Za-z0-9_]*)`")
	operationIn = regexp.MustCompile("`(GET|PUT|POST|DELETE|OPTIONS|HEAD|PATCH|TRACE) (/[^`\\s]*)`")
	separator   = regexp.MustCompile(`^:?-+:?$`)
	boldLeadIn  = regexp.MustCompile(`\*\*.*?\*\*`)
	codeSpan    = regexp.MustCompile("`[^`]*`")
	word        = regexp.MustCompile(`[A-Za-z]{2,}`)
)

// promiseFile is what one area's file in docs/promises says.
type promiseFile struct {
	rel        string
	operations []string
	promises   []promise
	// entries are the items under "Not reachable from outside".
	entries []entry
}

// promise is one row of an area's table.
type promise struct {
	id    string
	line  int
	tests []string
}

// entry is one item under "Not reachable from outside".
type entry struct {
	line int
	text string
}

// checkPromises checks that the promise files in docs/promises, the flows in
// test/flows and the operations in docs/openapi agree (TEST-18). It reports:
//   - a test a promise names that isn't a top-level test in test/flows;
//   - a promise that names no test;
//   - a test the "Not reachable" section names that no test in the module has;
//   - a flow no promise names;
//   - two promises with the same ID, or two flows with the same name;
//   - an operation in docs/openapi that's in no area's Operations line;
//   - a "Not reachable" entry with no reason, or no test;
//   - a module with flows or operations, and no docs/promises.
func checkPromises(m *module) ([]problem, error) {
	paths, err := filepath.Glob(filepath.Join(m.root, promisesDir, "*.md"))
	if err != nil {
		return nil, err
	}

	flows, tests, repeats := m.testsByKind()

	ops, err := m.operations()
	if err != nil {
		return nil, err
	}

	if len(paths) == 0 {
		if len(flows) == 0 && len(ops) == 0 {
			return nil, nil
		}

		return []problem{{path: promisesDir, msg: "missing: write one file per area, as .standards/templates/promises.md shows, listing every operation in docs/openapi and naming the flow that proves each promise (TEST-18)"}}, nil
	}

	var files []promiseFile

	for _, path := range paths {
		f, err := readPromiseFile(m.rel(path), path)
		if err != nil {
			return nil, err
		}

		files = append(files, f)
	}

	problems := checkPromiseFiles(files, flows, tests)

	for _, r := range repeats {
		first := r.first.fset.Position(r.first.pos)
		problems = append(problems, r.at.problem("%s is also a flow at %s:%d: a promise names its flow by name, so give each flow its own (TEST-18)", r.name, m.rel(first.Filename), first.Line))
	}
	named, covered := map[string]bool{}, map[string]bool{}

	for _, f := range files {
		for _, p := range f.promises {
			for _, t := range p.tests {
				named[t] = true
			}
		}

		for _, e := range f.entries {
			for _, t := range testsIn(e.text) {
				named[t] = true
			}
		}

		for _, op := range f.operations {
			covered[op] = true
		}
	}

	for _, name := range sortedKeys(flows) {
		if !named[name] {
			problems = append(problems, flows[name].problem("%s proves no promise: name it in its area's file in docs/promises, or delete it (TEST-18)", name))
		}
	}

	for _, op := range ops {
		if !covered[op.name] {
			problems = append(problems, problem{path: op.rel, line: op.line, msg: op.name + " is in no area's Operations line in docs/promises: add it to the area that covers it (TEST-18)"})
		}
	}

	return problems, nil
}

// checkPromiseFiles checks each promise and section entry against the flows and
// the module's tests.
func checkPromiseFiles(files []promiseFile, flows, tests map[string]place) []problem {
	var problems []problem

	ids := map[string]string{}

	for _, f := range files {
		at := func(line int, format string, args ...any) {
			problems = append(problems, problem{path: f.rel, line: line, msg: fmt.Sprintf(format, args...)})
		}

		for _, p := range f.promises {
			if first, ok := ids[p.id]; ok {
				at(p.line, "%s is also the ID of the promise at %s: give each promise its own (TEST-18)", p.id, first)
			} else {
				ids[p.id] = fmt.Sprintf("%s:%d", f.rel, p.line)
			}

			if len(p.tests) == 0 {
				at(p.line, "%s names no test in Proved by: name the flow that proves it (TEST-18)", p.id)
			}

			for _, t := range p.tests {
				_, isFlow := flows[t]
				_, exists := tests[t]

				switch {
				case isFlow:
				case exists:
					at(p.line, "%s names %s, which isn't a flow in test/flows: a promise is proved by a flow, as the caller meets it (TEST-18)", p.id, t)
				default:
					at(p.line, "%s names %s, and no flow in test/flows has that name: write the flow, or fix the name (TEST-18)", p.id, t)
				}
			}
		}

		for _, e := range f.entries {
			named := testsIn(e.text)

			for _, t := range named {
				if _, ok := tests[t]; !ok {
					at(e.line, "the %q entry names %s, and no test in the module has that name (TEST-18)", notReachable, t)
				}
			}

			if len(named) == 0 || !hasReason(e.text) {
				at(e.line, "a %q entry says why no caller can reach the promise, and names the lower test that covers it (TEST-18)", notReachable)
			}
		}
	}

	return problems
}

// readPromiseFile reads an area's Operations line, its table of promises, and
// the entries under "Not reachable from outside". It reads Markdown the way a renderer
// does: a code block holds examples, not promises; a table row may leave out
// its outer pipes, and \| is a pipe inside a cell; an entry is a top-level list
// item, and an indented line, nested items included, continues it.
func readPromiseFile(rel, path string) (promiseFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return promiseFile{}, err
	}

	f := promiseFile{rel: rel}

	var (
		section      string
		sectionLine  int
		fence        string
		inOperations bool
		idCol        = -1
		provedCol    = -1
	)

	for i, line := range strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n") {
		n, trimmed := i+1, strings.TrimSpace(line)

		if marks := fenceMarks(trimmed); fence == "" && len(marks) >= 3 {
			fence = marks
			continue
		} else if fence != "" {
			if strings.HasPrefix(marks, fence) && trimmed == marks {
				fence = ""
			}

			continue
		}

		// A line is a table row inside a table, or as its header. A list item
		// with a pipe in it is still a list item.
		item, isItem := listItem(line)
		cells, hasPipe := tableRow(trimmed)

		isRow := hasPipe && !isItem && (idCol >= 0 || cells[0] == "ID")
		if !isRow {
			idCol, provedCol = -1, -1
		}

		switch {
		case strings.HasPrefix(trimmed, "#"):
			section, sectionLine, inOperations = headingText(trimmed), n, false
		case strings.HasPrefix(trimmed, "**Operations:**"):
			inOperations = true
		case trimmed == "" || strings.HasPrefix(trimmed, "**") || isRow:
			inOperations = false
		}

		if inOperations {
			for _, op := range operationIn.FindAllStringSubmatch(line, -1) {
				f.operations = append(f.operations, op[1]+" "+op[2])
			}

			continue
		}

		switch {
		case isRow:
			switch {
			case idCol < 0 && cells[0] == "ID":
				idCol, provedCol = 0, slices.Index(cells, "Proved by")
			case idCol < 0 || provedCol < 0 || len(cells) <= provedCol || separator.MatchString(cells[0]):
			case cells[idCol] != "":
				f.promises = append(f.promises, promise{id: cells[idCol], line: n, tests: testsIn(cells[provedCol])})
			}
		case !strings.EqualFold(section, notReachable):
		case isItem:
			f.entries = append(f.entries, entry{line: n, text: item})
		case trimmed != "" && len(f.entries) > 0 && f.entries[len(f.entries)-1].line > sectionLine:
			f.entries[len(f.entries)-1].text += " " + trimmed
		}
	}

	return f, nil
}

var (
	closingHashes = regexp.MustCompile(`(^|\s+)#+$`)
	itemMarker    = regexp.MustCompile(`^(?:[-*+]|[0-9]{1,9}[.)])\s+`)
)

// headingText returns a heading's text, without its opening hashes or the
// closing ones CommonMark allows, as in "## Not reachable from outside ##".
func headingText(trimmed string) string {
	text := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))

	return strings.TrimSpace(closingHashes.ReplaceAllString(text, ""))
}

// fenceMarks returns the run of backticks or tildes a line starts with.
func fenceMarks(trimmed string) string {
	for _, mark := range []string{"`", "~"} {
		if rest := strings.TrimLeft(trimmed, mark); len(rest) < len(trimmed) {
			return trimmed[:len(trimmed)-len(rest)]
		}
	}

	return ""
}

// listItem returns the text of a top-level list item: a bullet or a number,
// indented by less than two spaces. A nested item belongs to the one above it.
func listItem(line string) (string, bool) {
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	if indent >= 2 || strings.HasPrefix(line, "\t") {
		return "", false
	}

	trimmed := strings.TrimSpace(line)

	marker := itemMarker.FindString(trimmed)
	if marker == "" {
		return "", false
	}

	return trimmed[len(marker):], true
}

// tableRow splits a Markdown table row into its trimmed cells, and says
// whether the line is one: it has a pipe that isn't escaped. The outer pipes
// are optional, and \| is a pipe inside a cell.
func tableRow(trimmed string) ([]string, bool) {
	var (
		cells   []string
		cell    strings.Builder
		escaped bool
		pipes   int
	)

	for _, r := range trimmed {
		switch {
		case escaped:
			if r != '|' {
				cell.WriteRune('\\')
			}

			cell.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == '|':
			pipes++
			cells = append(cells, strings.TrimSpace(cell.String()))
			cell.Reset()
		default:
			cell.WriteRune(r)
		}
	}

	if pipes == 0 {
		return nil, false
	}

	cells = append(cells, strings.TrimSpace(cell.String()))

	if strings.HasPrefix(trimmed, "|") {
		cells = cells[1:]
	}

	if strings.HasSuffix(trimmed, "|") && !strings.HasSuffix(trimmed, "\\|") {
		cells = cells[:len(cells)-1]
	}

	return cells, len(cells) > 0
}

func testsIn(text string) []string {
	var found []string

	for _, match := range namedTest.FindAllStringSubmatch(text, -1) {
		if !slices.Contains(found, match[1]) {
			found = append(found, match[1])
		}
	}

	return found
}

// hasReason says whether an entry says why, beyond its bold lead-in and the
// names in backticks: at least three words.
func hasReason(text string) bool {
	text = codeSpan.ReplaceAllString(boldLeadIn.ReplaceAllString(text, ""), "")

	return len(word.FindAllString(text, -1)) >= 3
}

// place is where a test is declared.
type place struct {
	fset *token.FileSet
	pos  token.Pos
	m    *module
}

func (p place) problem(format string, args ...any) problem {
	return p.m.at(p.fset, p.pos, format, args...)
}

// testsByKind returns the top-level tests in test/flows, and every test in the
// module, by name, and each flow whose name another flow already has. TestMain
// isn't a test. A test file this machine's build leaves out, such as a
// _windows_test.go file elsewhere, is read too.
func (m *module) testsByKind() (flows, all map[string]place, repeats []repeat) {
	flows, all = map[string]place{}, map[string]place{}

	visit := func(fset *token.FileSet, file *ast.File, rel string) {
		if !strings.HasSuffix(rel, "_test.go") {
			return
		}

		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Name.Name == "TestMain" || !isTestFunc(fn.Name.Name, "Test") {
				continue
			}

			at := place{fset: fset, pos: fn.Name.Pos(), m: m}
			all[fn.Name.Name] = at

			if !within(rel, flowsDir) {
				continue
			}

			if first, ok := flows[fn.Name.Name]; ok {
				repeats = append(repeats, repeat{name: fn.Name.Name, at: at, first: first})
				continue
			}

			flows[fn.Name.Name] = at
		}
	}

	m.eachFile(func(p *packages.Package, file *ast.File, rel string) { visit(p.Fset, file, rel) })

	for _, f := range m.ignoredFiles() {
		visit(f.fset, f.file, f.rel)
	}

	return flows, all, repeats
}

// repeat is a flow whose name another flow already has.
type repeat struct {
	name      string
	at, first place
}

// operationAt is one operation in a committed API description.
type operationAt struct {
	name string // METHOD /path
	rel  string
	line int
}

// operations returns every operation in the documents in docs/openapi (HTTP-7).
func (m *module) operations() ([]operationAt, error) {
	paths, err := filepath.Glob(filepath.Join(m.root, "docs", "openapi", "*.json"))
	if err != nil {
		return nil, err
	}

	var found []operationAt

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		var doc document
		if err := json.Unmarshal(data, &doc); err != nil {
			return nil, fmt.Errorf("%s: %w", m.rel(path), err)
		}

		for _, route := range sortedKeys(doc.Paths) {
			for _, method := range methods {
				if _, ok := doc.Paths[route][method]; ok {
					found = append(found, operationAt{name: strings.ToUpper(method) + " " + route, rel: m.rel(path), line: lineOf(string(data), route)})
				}
			}
		}
	}

	return found, nil
}

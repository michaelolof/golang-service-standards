package main

import (
	"cmp"
	"fmt"
	"go/token"
	"path/filepath"
	"slices"
)

// problem is one finding, at a path relative to the project root, and a line
// when there is one.
type problem struct {
	path string
	line int
	msg  string
}

func (p problem) String() string {
	if p.line == 0 {
		return p.path + ": " + p.msg
	}

	return fmt.Sprintf("%s:%d: %s", p.path, p.line, p.msg)
}

// at returns a problem at pos.
func (m *module) at(fset *token.FileSet, pos token.Pos, format string, args ...any) problem {
	position := fset.Position(pos)

	return problem{path: m.rel(position.Filename), line: position.Line, msg: fmt.Sprintf(format, args...)}
}

// rel returns path relative to the project root, with forward slashes.
func (m *module) rel(path string) string {
	rel, err := filepath.Rel(m.root, path)
	if err != nil {
		return path
	}

	return filepath.ToSlash(rel)
}

func sortProblems(problems []problem) {
	slices.SortFunc(problems, func(a, b problem) int {
		return cmp.Or(cmp.Compare(a.path, b.path), cmp.Compare(a.line, b.line), cmp.Compare(a.msg, b.msg))
	})
}

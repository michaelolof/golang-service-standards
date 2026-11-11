// Package comments checks comments in non-test code (STY-5).
//
//   - Punctuation is plain ASCII. A dash, curly quote, arrow or bullet looks
//     like its ASCII cousin but doesn't match it in a search. Letters, such as
//     the é in a name, are fine.
//   - A comment stands on its own: no references to plans, phases, tasks or
//     documents (§6.2, Phase 3, Task 12, improvements.md).
//   - No "Step N:" narration: a function that needs it is split into named steps.
//   - A TODO is TODO(TICKET-123): <action>.
//
// gocritic's commentedOutCode and docStub, and revive's exported and
// package-comments, check the rest of STY-5.
package comments

import (
	"go/ast"
	"go/token"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/michaelolof/golang-service-standards/lint/analyzers/internal/where"
	"golang.org/x/tools/go/analysis"
)

// Analyzer checks comments.
var Analyzer = &analysis.Analyzer{
	Name: "comments",
	Doc:  "checks that comments use ASCII punctuation, refer to no plans or steps, and give every TODO a ticket",
	Run:  run,
}

var (
	planReference = regexp.MustCompile(`§|\bPhase [0-9]|\bTask [0-9]+|\.md\b`)
	stepNarration = regexp.MustCompile(`(?m)^\s*Step [0-9]+:`)
	todo          = regexp.MustCompile(`\bTODO\b`)
	ticketedTODO  = regexp.MustCompile(`\bTODO\([A-Z][A-Z0-9]*-[0-9]+\): \S`)
	directive     = regexp.MustCompile(`^//(go:|nolint|lint:|line |export |extern |#)`)
)

func run(pass *analysis.Pass) (any, error) {
	if pkg, ok := where.Of(pass); ok && where.IsTestInfra(pkg.Rel, pkg.Name) {
		return nil, nil
	}

	for _, file := range where.Files(pass) {
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if !directive.MatchString(comment.Text) {
					check(pass, comment)
				}
			}
		}
	}

	return nil, nil
}

func check(pass *analysis.Pass, comment *ast.Comment) {
	text := comment.Text

	if i, r := nonASCIIPunctuation(text); i >= 0 {
		pass.Reportf(comment.Slash+token.Pos(i), "comments use plain ASCII punctuation: replace %q, so a search finds the text (STY-5)", r)
	}

	if m := planReference.FindString(text); m != "" {
		pass.Reportf(comment.Slash, "a comment stands on its own after the plan is gone: remove the reference to %q (STY-5)", m)
	}

	if stepNarration.MatchString(strings.TrimPrefix(strings.TrimPrefix(text, "//"), "/*")) {
		pass.Reportf(comment.Slash, "no Step N narration: split the function into named steps instead (STY-5)")
	}

	for _, line := range strings.Split(text, "\n") {
		if todo.MatchString(line) && !ticketedTODO.MatchString(line) {
			pass.Reportf(comment.Slash, "a TODO names its ticket and the action: TODO(TICKET-123): <action> (STY-5)")
			break
		}
	}
}

// nonASCIIPunctuation returns the byte offset and rune of the first character
// beyond ASCII that isn't a letter or a digit, or -1. § is left to the plan
// reference check, so it's reported once.
func nonASCIIPunctuation(text string) (int, rune) {
	for i, r := range text {
		if r >= utf8.RuneSelf && r != '§' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return i, r
		}
	}

	return -1, 0
}

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A rules table outside a code block is read. A table inside one, such as an
// example promise file, isn't, and a shorter fence inside a longer one doesn't
// end the block.
func TestRulesIn_SkipsTablesInCodeBlocks(t *testing.T) {
	doc := "# 99 · Example\n\n" +
		"| ID | Rule | Enforced by |\n|---|---|---|\n| EXA-1 | A real rule. | review |\n\n" +
		"```markdown\n| ORD-1 | An example promise. | — |\n```\n\n" +
		"````html\n```\n| ORD-2 | Still inside the outer block. | — |\n````\n\n" +
		"| EXA-2 | Another real rule. | review |\n"

	path := filepath.Join(t.TempDir(), "99-example.md")
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := rulesIn(path)
	if err != nil {
		t.Fatal(err)
	}

	want := "\n## [99 · Example](" + path + ")\n\n| ID | Rule |\n|---|---|\n| EXA-1 | A real rule. |\n| EXA-2 | Another real rule. |\n"
	if got != want {
		t.Fatalf("rulesIn =\n%s\nwant\n%s", got, want)
	}
}

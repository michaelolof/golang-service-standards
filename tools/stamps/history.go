package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// history is what check compares the working tree with. A stamp is new if the
// pushed code doesn't have it, and only new stamps are judged, because ERR-1
// leaves existing stamps alone.
type history struct {
	// tracked is false outside git. Then every stamp counts as new, and where a
	// stamp came from can't be checked.
	tracked bool
	// known counts each stamp in the pushed code, so a stamp that was already
	// repeated there isn't reported, and a moved line isn't new.
	known map[string]int
	// issued holds every stamp fill has handed out on this clone.
	issued map[string]bool
}

// check lists what's wrong: a placeholder nobody filled, a stamp from the future
// (typed, not read from a clock), a new repeat, or a new stamp fill didn't hand out.
func check(root string, nowMillis int64, past history) ([]string, error) {
	uses, err := allStamps(root)
	if err != nil {
		return nil, err
	}

	places := map[string][]string{}

	var problems []string

	for _, use := range uses {
		if use.test {
			continue
		}

		places[use.value] = append(places[use.value], use.place)

		if value, err := strconv.ParseInt(use.value, 10, 64); err == nil && len(use.value) == 13 && value > nowMillis {
			problems = append(problems, fmt.Sprintf("stamp %s at %s is in the future, so it didn't come from a clock", use.value, use.place))
			continue
		}

		if past.tracked && past.known[use.value] == 0 && !past.issued[use.value] {
			problems = append(problems, fmt.Sprintf("stamp %s at %s wasn't handed out by task stamps", use.value, use.place))
		}
	}

	for value, where := range places {
		if len(where) > 1 && len(where) > past.known[value] {
			problems = append(problems, fmt.Sprintf("stamp %s is used %d times: %s", value, len(where), strings.Join(where, ", ")))
		}
	}

	files, err := goFiles(root)
	if err != nil {
		return nil, err
	}

	for _, path := range files {
		text, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}

		if bytes.Contains(text, []byte(placeholder)) {
			relative, _ := filepath.Rel(root, path)
			problems = append(problems, fmt.Sprintf("%s still has %s: run task stamps", filepath.ToSlash(relative), placeholder))
		}
	}

	sort.Strings(problems)

	return problems, nil
}

// loadHistory reads the pushed code's stamps and the record of handed-out stamps.
func loadHistory(root string) (history, error) {
	if !succeeds(root, "rev-parse", "--git-dir") {
		return history{}, nil
	}

	bases, err := pushedBases(root)
	if err != nil {
		return history{}, err
	}

	known := map[string]int{}

	for _, base := range bases {
		counts, err := stampsAt(root, base)
		if err != nil {
			return history{}, err
		}

		for value, n := range counts {
			known[value] = max(known[value], n)
		}
	}

	issued, err := readRecord(recordFile(root))
	if err != nil {
		return history{}, err
	}

	return history{tracked: true, known: known, issued: issued}, nil
}

// pushedBases returns the commits that stand for "the last push": the pushed
// commits the unpushed ones build on, or HEAD when nothing is unpushed. A
// project with no remote has nothing pushed, so HEAD stands in, and only
// uncommitted work is new. With no commits at all, everything is new.
func pushedBases(root string) ([]string, error) {
	if !succeeds(root, "rev-parse", "--verify", "-q", "HEAD") {
		return nil, nil
	}

	remotes, err := git(root, "for-each-ref", "--format=%(objectname)", "refs/remotes")
	if err != nil {
		return nil, err
	}

	if remotes == "" {
		return []string{"HEAD"}, nil
	}

	unpushed, err := git(root, "rev-list", "--boundary", "HEAD", "--not", "--remotes")
	if err != nil {
		return nil, err
	}

	if unpushed == "" {
		return []string{"HEAD"}, nil
	}

	var bases []string

	for line := range strings.SplitSeq(unpushed, "\n") {
		if commit, ok := strings.CutPrefix(line, "-"); ok {
			bases = append(bases, commit)
		}
	}

	return bases, nil
}

// stampsAt counts each stamp in a commit's Go files, outside tests. git lists only
// the files under the project's root, relative to it.
func stampsAt(root, commit string) (map[string]int, error) {
	listing, err := git(root, "ls-tree", "-r", "--name-only", commit)
	if err != nil {
		return nil, err
	}

	var paths []string

	for path := range strings.SplitSeq(listing, "\n") {
		if strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") && !skipped(path) {
			paths = append(paths, path)
		}
	}

	counts := map[string]int{}

	if len(paths) == 0 {
		return counts, nil
	}

	var request strings.Builder
	for _, path := range paths {
		request.WriteString(commit + ":./" + path + "\n") // ./ keeps the path relative to the project, which may be a subdirectory
	}

	cmd := exec.Command("git", "-C", root, "cat-file", "--batch")
	cmd.Stdin = strings.NewReader(request.String())

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git cat-file: %w", err)
	}

	reader := bufio.NewReader(bytes.NewReader(out))

	for _, path := range paths {
		header, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("git cat-file %s: %w", path, err)
		}

		fields := strings.Fields(header)
		if len(fields) != 3 {
			return nil, fmt.Errorf("git cat-file %s: unexpected %q", path, header)
		}

		size, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("git cat-file %s: %w", path, err)
		}

		src := make([]byte, size+1) // the content, then a newline
		if _, err := io.ReadFull(reader, src); err != nil {
			return nil, fmt.Errorf("git cat-file %s: %w", path, err)
		}

		uses, err := stampsIn(path, src[:size])
		if err != nil {
			continue // Code that didn't parse when it was pushed has no stamps to compare with.
		}

		for _, use := range uses {
			counts[use.value]++
		}
	}

	return counts, nil
}

// recordFile is where fill records what it hands out: in the git directory, so
// it's never committed and every worktree of a clone shares it. Outside git there's
// no record.
func recordFile(root string) string {
	dir, err := git(root, "rev-parse", "--git-common-dir")
	if err != nil {
		return ""
	}

	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}

	return filepath.Join(dir, "stamps-issued")
}

func readRecord(path string) (map[string]bool, error) {
	issued := map[string]bool{}

	text, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return issued, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	for line := range strings.SplitSeq(string(text), "\n") {
		if line != "" {
			issued[line] = true
		}
	}

	return issued, nil
}

// record adds handed-out stamps to the record. It runs before any file is
// written, so a stamp in the code is always in the record.
func record(path string, stamps []int64) error {
	if path == "" || len(stamps) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}

	var lines strings.Builder
	for _, stamp := range stamps {
		lines.WriteString(strconv.FormatInt(stamp, 10) + "\n")
	}

	if _, err := file.WriteString(lines.String()); err != nil {
		_ = file.Close()
		return fmt.Errorf("write %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}

// succeeds says whether a git command exits cleanly, for questions like "is this a repo?".
func succeeds(root string, args ...string) bool {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Run() == nil
}

func git(root string, args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(string(out)), nil
}

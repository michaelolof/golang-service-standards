package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

const header = "package a\n\nimport \"example.com/x/errx\"\n\n"

func repoWith(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	write(t, root, files)

	return root
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()

	for name, text := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func read(t *testing.T, path string) string {
	t.Helper()

	text, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(text)
}

// scriptedClock returns the given readings in turn, then keeps counting up.
func scriptedClock(readings ...int64) clock {
	next := 0

	return clock{
		nowMillis: func() int64 {
			if next < len(readings) {
				next++
				return readings[next-1]
			}

			next++

			return readings[len(readings)-1] + int64(next)
		},
		sleep:   func(time.Duration) {},
		shuffle: func(int, func(i, j int)) {},
	}
}

func wantProblems(t *testing.T, got []string, err error, want ...string) {
	t.Helper()

	if err != nil {
		t.Fatal(err)
	}

	if len(got) == 0 && len(want) == 0 {
		return
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("problems:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestFill_SkipsAReadingAlreadyInTheProject(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": header + "var _ = errx.New(1793000000001, \"old\")\nvar _ = errx.New(__STAMP__, \"new\")\n"})

	filled, err := fill(root, scriptedClock(1793000000001, 1793000000002), "")
	if err != nil || filled != 1 {
		t.Fatalf("filled %d, %v", filled, err)
	}

	if !strings.Contains(read(t, filepath.Join(root, "a.go")), "errx.New(1793000000002, \"new\")") {
		t.Fatal("the placeholder didn't get the second reading")
	}
}

func TestFill_TheSameMillisecondTwiceGivesTwoStamps(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": header + "var _ = errx.New(__STAMP__, \"a\")\nvar _ = errx.New(__STAMP__, \"b\")\n"})

	if _, err := fill(root, scriptedClock(1793000000005, 1793000000005, 1793000000006), ""); err != nil {
		t.Fatal(err)
	}

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err)
}

func TestFill_SkipsAStampUsedInATest(t *testing.T) {
	root := repoWith(t, map[string]string{
		"a_test.go": header + "var _ = errx.New(1793000000001, \"test\")\n",
		"a.go":      header + "var _ = errx.New(__STAMP__, \"new\")\n",
	})

	if _, err := fill(root, scriptedClock(1793000000001, 1793000000002), ""); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(read(t, filepath.Join(root, "a.go")), "1793000000002") {
		t.Fatal("the placeholder didn't get the second reading")
	}
}

func TestFill_RefusesAPlaceholderInATest(t *testing.T) {
	root := repoWith(t, map[string]string{"a_test.go": header + "var _ = errx.New(__STAMP__, \"test\")\n"})

	_, err := fill(root, scriptedClock(1793000000001), "")
	if err == nil || !strings.Contains(err.Error(), "tests use 0") {
		t.Fatalf("got %v", err)
	}
}

func TestFill_RecordsWhatItHandsOut(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": header + "var _ = errx.New(__STAMP__, \"a\")\nvar _ = errx.New(__STAMP__, \"b\")\n"})
	recordPath := filepath.Join(t.TempDir(), "stamps-issued")

	if _, err := fill(root, scriptedClock(1793000000001, 1793000000002), recordPath); err != nil {
		t.Fatal(err)
	}

	if got := read(t, recordPath); got != "1793000000001\n1793000000002\n" {
		t.Fatalf("record: %q", got)
	}
}

func TestLock_OneRunAtATime(t *testing.T) {
	root := t.TempDir()

	release, err := lock(root, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := lock(root, 50*time.Millisecond); err == nil || !strings.Contains(err.Error(), "another stamps run") {
		t.Fatalf("got %v", err)
	}

	release()

	release, err = lock(root, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	release()
}

func TestCheck_ReportsARepeat(t *testing.T) {
	root := repoWith(t, map[string]string{
		"a.go": header + "var _ = errx.New(1793000000001, \"a\")\n",
		"b.go": header + "var _ = errx.New(1793000000001, \"b\")\n",
	})

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err, "stamp 1793000000001 is used 2 times: a.go:5, b.go:5")
}

func TestCheck_FindsAStampUnderAnImportAlias(t *testing.T) {
	aliased := "package a\n\nimport e \"example.com/x/errx\"\n\nvar _ = e.New(1793000000001, \"a\")\n"
	root := repoWith(t, map[string]string{"a.go": aliased, "b.go": header + "var _ = errx.New(1793000000001, \"b\")\n"})

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err, "stamp 1793000000001 is used 2 times: a.go:5, b.go:5")
}

func TestCheck_ReportsAStampFromTheFuture(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": header + "var _ = errx.New(1793000000099, \"a\")\n"})

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err, "stamp 1793000000099 at a.go:5 is in the future, so it didn't come from a clock")
}

func TestCheck_ReportsAnUnfilledPlaceholder(t *testing.T) {
	root := repoWith(t, map[string]string{"a.go": header + "var _ = errx.New(__STAMP__, \"a\")\n"})

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err, "a.go still has __STAMP__: run task stamps")
}

func TestCheck_IgnoresZeroAndRepeatsInTests(t *testing.T) {
	root := repoWith(t, map[string]string{
		"a.go":      header + "var _ = errx.New(1793000000001, \"a\")\n",
		"a_test.go": header + "var _ = errx.New(0, \"a\")\nvar _ = errx.New(0, \"b\")\nvar _ = errx.New(1793000000001, \"c\")\n",
	})

	problems, err := check(root, 1793000000009, history{})
	wantProblems(t, problems, err)
}

func TestCheck_JudgesOnlyNewStamps(t *testing.T) {
	root := repoWith(t, map[string]string{
		"old.go":    header + "var _ = errx.New(1793000000001, \"a\")\nvar _ = errx.New(1793000000001, \"b\")\n",
		"filled.go": header + "var _ = errx.New(1793000000005, \"c\")\n",
		"typed.go":  header + "var _ = errx.New(1793000000002, \"d\")\nvar _ = errx.New(1793000000003, \"e\")\n",
	})
	past := history{
		tracked: true,
		known:   map[string]int{"1793000000001": 2},
		issued:  map[string]bool{"1793000000005": true},
	}

	problems, err := check(root, 1793000000009, past)
	wantProblems(t, problems, err,
		"stamp 1793000000002 at typed.go:5 wasn't handed out by task stamps",
		"stamp 1793000000003 at typed.go:6 wasn't handed out by task stamps",
	)
}

func TestCheck_ReportsACopyOfAnOldStamp(t *testing.T) {
	root := repoWith(t, map[string]string{
		"a.go": header + "var _ = errx.New(1793000000001, \"a\")\n",
		"b.go": header + "var _ = errx.New(1793000000001, \"copied\")\n",
	})
	past := history{tracked: true, known: map[string]int{"1793000000001": 1}}

	problems, err := check(root, 1793000000009, past)
	wantProblems(t, problems, err, "stamp 1793000000001 is used 2 times: a.go:5, b.go:5")
}

// The tests below run git, as the tasks do.

func gitIn(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com", "-c", "commit.gpgsign=false"}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// pushedProject makes a project with one pushed commit holding old.go.
func pushedProject(t *testing.T) string {
	t.Helper()

	remote := t.TempDir()
	gitIn(t, remote, "init", "-q", "--bare")

	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	write(t, root, map[string]string{"old.go": header + "var _ = errx.New(1793000000001, \"a\")\nvar _ = errx.New(1793000000001, \"b\")\nvar _ = errx.New(1793000000002, \"moves\")\n"})
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "old")
	gitIn(t, root, "remote", "add", "origin", remote)
	gitIn(t, root, "push", "-q", "-u", "origin", "main")

	return root
}

func checkProject(t *testing.T, root string) ([]string, error) {
	t.Helper()

	past, err := loadHistory(root)
	if err != nil {
		return nil, err
	}

	return check(root, time.Now().UnixMilli(), past)
}

func TestGit_PushedStampsAreLeftAlone(t *testing.T) {
	root := pushedProject(t)

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err)
}

func TestGit_ATypedStampIsReportedAndAFilledOneIsNot(t *testing.T) {
	root := pushedProject(t)
	write(t, root, map[string]string{
		"typed.go":  header + "var _ = errx.New(1793000000003, \"typed\")\n",
		"filled.go": header + "var _ = errx.New(__STAMP__, \"filled\")\n",
	})

	if _, err := fill(root, realClock(), recordFile(root)); err != nil {
		t.Fatal(err)
	}

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err, "stamp 1793000000003 at typed.go:5 wasn't handed out by task stamps")
}

func TestGit_AnUnpushedCommitIsStillChecked(t *testing.T) {
	root := pushedProject(t)
	write(t, root, map[string]string{"typed.go": header + "var _ = errx.New(1793000000003, \"typed\")\n"})
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "typed")

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err, "stamp 1793000000003 at typed.go:5 wasn't handed out by task stamps")

	gitIn(t, root, "push", "-q")

	problems, err = checkProject(t, root)
	wantProblems(t, problems, err)
}

func TestGit_AMovedLineIsNotNew(t *testing.T) {
	root := pushedProject(t)
	write(t, root, map[string]string{
		"old.go": header + "var _ = errx.New(1793000000001, \"a\")\nvar _ = errx.New(1793000000001, \"b\")\n",
		"new.go": header + "var _ = errx.New(1793000000002, \"moves\")\n",
	})

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err)
}

func TestGit_WithNoRemoteCommittedStampsAreLeftAlone(t *testing.T) {
	root := t.TempDir()
	gitIn(t, root, "init", "-q", "-b", "main")
	write(t, root, map[string]string{"old.go": header + "var _ = errx.New(1793000000001, \"a\")\n"})
	gitIn(t, root, "add", ".")
	gitIn(t, root, "commit", "-q", "-m", "old")
	write(t, root, map[string]string{"typed.go": header + "var _ = errx.New(1793000000003, \"typed\")\n"})

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err, "stamp 1793000000003 at typed.go:5 wasn't handed out by task stamps")
}

func TestGit_AProjectInASubdirectory(t *testing.T) {
	repo := t.TempDir()
	gitIn(t, repo, "init", "-q", "-b", "main")
	write(t, repo, map[string]string{
		"service/old.go": header + "var _ = errx.New(1793000000001, \"a\")\n",
		"other/x.go":     header + "var _ = errx.New(1793000000003, \"elsewhere\")\n",
	})
	gitIn(t, repo, "add", ".")
	gitIn(t, repo, "commit", "-q", "-m", "old")

	root := filepath.Join(repo, "service")
	write(t, root, map[string]string{"typed.go": header + "var _ = errx.New(1793000000003, \"typed\")\n"})

	problems, err := checkProject(t, root)
	wantProblems(t, problems, err, "stamp 1793000000003 at typed.go:5 wasn't handed out by task stamps")
}

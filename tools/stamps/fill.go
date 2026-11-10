package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// clock is where stamps come from, so a test can control the readings.
type clock struct {
	nowMillis func() int64
	sleep     func(time.Duration)
	// shuffle puts the readings in a random order across the placeholders.
	shuffle func(n int, swap func(i, j int))
}

func realClock() clock {
	return clock{nowMillis: func() int64 { return time.Now().UnixMilli() }, sleep: time.Sleep, shuffle: rand.Shuffle}
}

// fill replaces every placeholder with its own clock reading, and records each one
// in recordPath. A reading is only used if no stamp anywhere in the project already
// has it, tests included. Readings are spaced out and shuffled, because ERR-1 says a
// stamp has no relation to its neighbours.
func fill(root string, c clock, recordPath string) (int, error) {
	files, err := goFiles(root)
	if err != nil {
		return 0, err
	}

	counts := map[string]int{}
	total := 0

	for _, path := range files {
		text, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", path, err)
		}

		n := strings.Count(string(text), placeholder)
		if n == 0 {
			continue
		}

		if strings.HasSuffix(path, "_test.go") {
			return 0, fmt.Errorf("%s: tests use 0, not %s", path, placeholder)
		}

		counts[path] = n
		total += n
	}

	uses, err := allStamps(root)
	if err != nil {
		return 0, err
	}

	used := make(map[int64]bool, len(uses))

	for _, use := range uses {
		value, err := strconv.ParseInt(use.value, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("stamp %s at %s: %w", use.value, use.place, err)
		}

		used[value] = true
	}

	readings := takeReadings(total, used, c)
	c.shuffle(len(readings), func(i, j int) { readings[i], readings[j] = readings[j], readings[i] })

	if err := record(recordPath, readings); err != nil {
		return 0, err
	}

	next := 0

	for path, n := range counts {
		text, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("read %s: %w", path, err)
		}

		updated := string(text)
		for range n {
			updated = strings.Replace(updated, placeholder, strconv.FormatInt(readings[next], 10), 1)
			next++
		}

		if err := writeFile(path, updated); err != nil {
			return 0, err
		}
	}

	return total, nil
}

// takeReadings reads the clock once per stamp, and waits if a reading is already taken.
func takeReadings(n int, used map[int64]bool, c clock) []int64 {
	readings := make([]int64, 0, n)

	for len(readings) < n {
		value := c.nowMillis()
		if used[value] {
			c.sleep(time.Millisecond)
			continue
		}

		used[value] = true
		readings = append(readings, value)

		c.sleep(time.Duration(25+rand.IntN(50)) * time.Millisecond)
	}

	return readings
}

// writeFile replaces a file in one step, so a crash can't leave half of it.
func writeFile(path, text string) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	temp := path + ".stamps"
	if err := os.WriteFile(temp, []byte(text), info.Mode()); err != nil {
		return fmt.Errorf("write %s: %w", temp, err)
	}

	if err := os.Rename(temp, path); err != nil {
		return fmt.Errorf("replace %s: %w", path, err)
	}

	return nil
}

// lock lets one fill run at a time per project, so two runs can't take the same reading.
// fill reads the repo's stamps after this returns, so it sees what the last run wrote.
func lock(root string, timeout time.Duration) (func(), error) {
	sum := sha256.Sum256([]byte(root))
	path := filepath.Join(os.TempDir(), "stamps-"+hex.EncodeToString(sum[:8])+".lock")
	deadline := time.Now().Add(timeout)

	for {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}

		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("lock: %w", err)
		}

		if info, statErr := os.Stat(path); statErr == nil && time.Since(info.ModTime()) > staleLock {
			_ = os.Remove(path)
			continue
		}

		if time.Now().After(deadline) {
			return nil, fmt.Errorf("another stamps run holds %s", path)
		}

		time.Sleep(10 * time.Millisecond)
	}
}

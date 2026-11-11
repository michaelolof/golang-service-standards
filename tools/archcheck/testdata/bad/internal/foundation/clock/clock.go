// Package clock tells the time.
package clock

// Clock is the shared clock.
type Clock interface{ Now() int64 }

// Unwatched is declared and never used.
type Unwatched interface{ Watch() }

// System reads the system clock.
type System struct{}

func (System) Now() int64 { return 0 }

// ticker is a consumer-owned interface, used only in this package.
type ticker interface{ Tick() }

type everySecond struct{}

func (everySecond) Tick() {}

func start(t ticker) { t.Tick() }

var _ = func() int { start(everySecond{}); return 0 }()

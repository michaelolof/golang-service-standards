// Package payout pays out.
package payout

// Clock duplicates foundation's clock.
type Clock interface{ Now() int64 }

// Run uses its clock.
func Run(c Clock) int64 { return c.Now() }

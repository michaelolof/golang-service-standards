// Package txboundary marks the ports whose writes may join a use case's
// transaction (DATA-4).
package txboundary

// Port marks a boundary port. It adds no methods.
type Port any

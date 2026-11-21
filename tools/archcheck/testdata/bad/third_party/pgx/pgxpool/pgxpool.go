// Package pgxpool is a stand-in for the real driver.
package pgxpool

import "context"

// Pool is a pool of connections.
type Pool struct{}

// Row is one row a query returns.
type Row interface{ Scan(dest ...any) error }

// Exec runs a statement.
func (p *Pool) Exec(ctx context.Context, sql string, args ...any) (string, error) { return "", nil }

// QueryRow runs a query that returns one row.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) Row { return nil }

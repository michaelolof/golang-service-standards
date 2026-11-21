// Package orders stores orders.
package orders

import "context"

// Repo stores orders.
type Repo struct{}

// New builds a Repo.
func New() *Repo { return &Repo{} }

// Place stores a placed order.
func (r *Repo) Place(ctx context.Context, id string) error { return nil }

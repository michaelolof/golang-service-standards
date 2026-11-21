// Package admins stores the operators who sign in to the admin API.
package admins

import "context"

// Repo stores admins.
type Repo struct{}

// New builds a Repo.
func New() *Repo { return &Repo{} }

// Create stores a new admin.
func (r *Repo) Create(ctx context.Context, email string) error { return nil }

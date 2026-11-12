package audits

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ ports.AuditWriter = (*Repo)(nil)

// Repo writes audit entries. It joins the caller's transaction.
type Repo struct{ db *pgxpool.Pool }

func New(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Record(ctx context.Context, entry domain.AuditEntry) error {
	_, err := r.db.Exec(ctx, `INSERT INTO audit_entries (action) VALUES ($1)`, entry.Action)

	return err
}

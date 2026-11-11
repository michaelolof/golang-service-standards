package transfers

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/gocql/gocql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const transferColumns = `id, status`

type Repo struct {
	db      *pgxpool.Pool
	sql     *sql.DB
	session *gocql.Session
}

func (r *Repo) Get(ctx context.Context, id string) error {
	const q = `SELECT ` + transferColumns + ` FROM transfers WHERE id = @id`

	_, err := r.db.Query(ctx, q, pgx.NamedArgs{"id": id})

	return err
}

func (r *Repo) List(ctx context.Context, where string) error {
	_, err := r.db.Query(ctx, "SELECT id FROM transfers WHERE "+where) // want `a statement is a constant, declared in the method that runs it: join constants with \+, and write optional filters as \(@x::T IS NULL OR col = @x\) \(DATA-5\)`
	if err != nil {
		return err
	}

	_, err = r.sql.QueryContext(ctx, fmt.Sprintf("SELECT %s FROM t", transferColumns)) // want `a statement is a constant`

	return err
}

func (r *Repo) Touch(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `UPDATE transfers SET updated_at = now() WHERE id = @id`) // want `time comes from the clock: pass now as a parameter instead of now\(\) \(DATA-6\)`

	return err
}

func (r *Repo) CreateOrGet(ctx context.Context) error {
	const q = `INSERT INTO transfers (id) VALUES (@id) ON CONFLICT DO NOTHING`

	_, err := r.db.Exec(ctx, q) // want `name the conflict target, ON CONFLICT \(cols\) DO …, so another unique constraint's violation isn't swallowed \(DATA-8\)`

	return err
}

func (r *Repo) Upsert(ctx context.Context) error {
	const q = `INSERT INTO transfers (id) VALUES (@id) ON CONFLICT (org_id, id) DO NOTHING`

	_, err := r.db.Exec(ctx, q)
	_ = r.session.Query(`SELECT id FROM transfers_by_id WHERE id = ?`)

	return err
}

func (r *Repo) MarkSeen() map[string]any {
	return map[string]any{"$currentDate": map[string]any{"seen_at": true}} // want `time comes from the clock: set the field to now from clock.Clock, not \$currentDate \(DATA-6\)`
}

package transfers

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Transfer struct{ ID [16]byte } // want `Repo is a repository's only exported type: Transfer goes in domain if callers need it, or becomes unexported in row.go \(DATA-1, DATA-2\)`

type Repo struct{ db *pgxpool.Pool }

// New may take the driver's handle.
func New(db *pgxpool.Pool) *Repo { return &Repo{db: db} }

func (r *Repo) Get(ctx context.Context, id [16]byte) (Transfer, error) { return Transfer{}, nil }

func (r *Repo) CreateTx(ctx context.Context, tx pgx.Tx, t Transfer) error { return nil } // want `Repo.CreateTx takes or returns a type from github.com/jackc/pgx/v5: repository methods take and return domain types \(DATA-2\)`

func (r *Repo) scan(row pgx.Row) (Transfer, error) { return Transfer{}, nil }

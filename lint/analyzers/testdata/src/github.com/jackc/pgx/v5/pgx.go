package pgx

type Tx interface{ Commit() error }

type Rows interface{ Next() bool }

type Row interface{ Scan(dest ...any) error }

type NamedArgs map[string]any

type Batch struct{}

func (b *Batch) Queue(query string, args ...any) {}

// A store adapter hands its driver's handle to repositories.
package crdbstore

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type Config struct{ URL string }

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	_ = ctx

	return &pgxpool.Pool{}, nil
}

func WithinTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error { return nil }

func Cache() *goredis.Client { return nil } // want `Cache exposes a type from github.com/redis/go-redis/v9`

package crdbstore

import "github.com/jackc/pgx/v5/pgxpool"

const Name = "crdbstore"

var _ pgxpool.Pool

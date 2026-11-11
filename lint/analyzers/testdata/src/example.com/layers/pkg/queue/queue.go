package queue

import (
	"example.com/layers/internal/domain"
	"github.com/jackc/pgx/v5"
)

const Name = "queue"

var (
	_ domain.Transfer
	_ pgx.Tx
)

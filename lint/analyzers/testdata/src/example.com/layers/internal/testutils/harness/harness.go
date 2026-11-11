package harness

import (
	"example.com/layers/internal/adapters/redis"
	goredis "github.com/redis/go-redis/v9"
)

var _ = redis.Name
var _ goredis.Client

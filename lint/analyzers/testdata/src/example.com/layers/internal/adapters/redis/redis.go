package redis

import goredis "github.com/redis/go-redis/v9"

const Name = "redis"

var _ goredis.Client

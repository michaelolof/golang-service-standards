package kafka

import (
	"example.com/layers/internal/adapters/kafka/codec"
	"example.com/layers/internal/adapters/redis" // want `internal/adapters/kafka may not import internal/adapters/redis: adapters imports only domain, ports, jobs, foundation and core \(LAY-7\)`
	"example.com/layers/internal/core/other"
	"github.com/aws/aws-sdk-go-v2/aws"
	goredis "github.com/redis/go-redis/v9" // want `github.com/redis/go-redis/v9 belongs in internal/adapters/redis: each SDK has one home \(LAY-7\)`
	"github.com/twmb/franz-go/pkg/kgo"
)

const Name = "kafka"

var (
	_ = codec.Name
	_ = redis.Name
	_ = other.Name
	_ aws.Config
	_ goredis.Client
	_ kgo.Client
)

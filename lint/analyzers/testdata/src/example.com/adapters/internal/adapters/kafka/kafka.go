package kafka

import (
	"context"

	"github.com/twmb/franz-go/pkg/kgo"
)

type ProducerConfig struct{ Brokers []string }

type Producer struct{ client *kgo.Client }

func New(ctx context.Context, cfg ProducerConfig) (*Producer, error) { // want `New takes a ctx only when building the client needs one, and this one is never used \(ABS-7\)`
	return &Producer{client: &kgo.Client{}}, nil
}

func Open(brokers []string) (*Producer, error) { return nil, nil } // want `an adapter is built by Open\(cfg Config\) \(\*T, error\), taking its own Config struct \(ABS-7\)`

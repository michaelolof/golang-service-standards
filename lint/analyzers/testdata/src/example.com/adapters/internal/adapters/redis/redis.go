package redis

import (
	"context"

	goredis "github.com/redis/go-redis/v9"
)

type Config struct{ Addr string }

type Client struct {
	Raw  *goredis.Client // want `Client.Raw has a type from github.com/redis/go-redis/v9: keep the SDK's types unexported \(ABS-7\)`
	conn *goredis.Client
}

type Lock struct{ client *goredis.Client }

// New uses its ctx, so it may take one.
func New(ctx context.Context, cfg Config) (*Client, error) {
	_ = ctx.Err()

	return &Client{conn: &goredis.Client{}}, nil
}

// NewLock is a role constructor: it takes the one client, not a Config.
func NewLock(client *Client, ttl int) *Lock { return &Lock{client: client.conn} }

func (c *Client) Options() goredis.Options { return goredis.Options{} } // want `Options exposes a type from github.com/redis/go-redis/v9: an adapter wraps its SDK and leaks no SDK type \(ABS-7\)`

func (c *Client) Ping(ctx context.Context) error { return nil }

func (l *Lock) raw() *goredis.Client { return l.client }

type Nil = goredis.Client // want `Nil is built from a type in github.com/redis/go-redis/v9: an adapter leaks no SDK type \(ABS-7\)`

// Package mailer sends mail.
package mailer

import "example.com/bad/internal/ports"

var _ ports.Mailer = (*Client)(nil)

// Client says which port it implements.
type Client struct{}

// New builds a Client.
func New() *Client { return &Client{} }

func (c *Client) Send() error { return nil }

func (c *Client) Share() {}

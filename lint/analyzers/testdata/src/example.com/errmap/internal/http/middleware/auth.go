// Package middleware declares no handlers, so its helpers aren't checked.
package middleware

import (
	"errors"

	"github.com/michaelolof/gofi"
)

func authenticate(c gofi.Context, token string) error {
	if token == "" {
		return errors.New("missing token")
	}

	return c.Next()
}

var _ = authenticate

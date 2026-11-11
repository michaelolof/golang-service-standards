// Package errkinds lists every application error kind.
package errkinds

import "github.com/michaelolof/errx"

// ErrKindNotFound: core returns early on it.
var ErrKindNotFound = errx.Kind("not_found")

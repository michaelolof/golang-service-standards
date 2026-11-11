// Package errkinds lists every application error kind.
package errkinds

import "github.com/michaelolof/errx"

var (
	ErrKindNotFound = errx.Kind("not_found")
	ErrKindConflict = errx.Kind("conflict")
	ErrKindUnused   = errx.Kind("unused")
	ErrKindChecked  = errx.Kind("checked")
	ErrKindMapped   = errx.Kind("mapped")
)

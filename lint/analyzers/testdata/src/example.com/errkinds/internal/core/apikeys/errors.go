package apikeys

import "github.com/michaelolof/errx"

var ErrKindKeyNotFound = errx.Kind("apikeys_key_not_found") // want `every application kind is declared in internal/foundation/errkinds, and nowhere else: reuse a general kind, or add one there \(ERR-5\)`

// Using a kind anywhere is fine.
func missing(err error) bool { return errx.IsKind(err, ErrKindKeyNotFound) }

var _ = missing

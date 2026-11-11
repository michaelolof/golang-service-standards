package errkinds

import "github.com/michaelolof/errx"

// ErrKindNotFound: core and transports branch on it.
var ErrKindNotFound = errx.Kind("not_found")

var ErrKindConflict = errx.Kind("conflict")

var NotFoundErr = errx.Kind("missing") // want `a kind is named ErrKind<Name>: rename NotFoundErr \(ERR-5\)`

var ErrKindRateLimited = errx.Kind("rate-limited") // want `a kind's string is snake_case: "rate-limited" \(ERR-5\)`

var ErrKindDuplicate = errx.Kind("conflict") // want `ErrKindDuplicate has the same string as ErrKindConflict: errx compares kinds by string \(ERR-5\)`

var name = "dynamic"

var ErrKindDynamic = errx.Kind(name) // want `a kind's string is a literal, so the list shows every kind \(ERR-5\)`

func local() any { return errx.Kind("local") } // want `a kind is declared as a package variable`

var _ = local

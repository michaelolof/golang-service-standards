// Package httperr maps kinds to statuses.
package httperr

import "example.com/bad/internal/foundation/errkinds"

// statuses is a mapping table, which counts as using a kind.
var statuses = map[any]int{errkinds.ErrKindMapped: 400}

// Status returns a kind's status.
func Status(kind any) int { return statuses[kind] }

// A test helper may sit below a flat layer, and import anything.
package domaintest

import (
	"example.com/layers/internal/core/other"
	"example.com/layers/internal/domain"
)

var _ = other.Name

func Transfer() domain.Transfer { return domain.Transfer{} }

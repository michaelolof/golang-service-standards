// Code at the repository root is reported by the repos analyzer, so layers
// leaves it, and its imports, alone.
package repository

import (
	"example.com/layers/internal/repository/accounts"
	"example.com/layers/internal/repository/transfers"
)

var _, _ = accounts.Name, transfers.Name

// Package migrations embeds the migrations.
package migrations

import "embed"

//go:embed *.sql
var Files embed.FS

package migrations

import "embed"

// files contains the immutable, ordered migration sources.
//
//go:embed *.up.sql
var files embed.FS

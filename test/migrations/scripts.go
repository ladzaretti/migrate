package migrations

import (
	_ "embed"
)

//go:embed 000_init.sql
var _000 string

//go:embed 001_init_copy.sql
var _001 string

func Scripts() []string {
	return []string{
		_000,
		_001,
	}
}

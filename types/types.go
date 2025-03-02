package types

import (
	"context"
	"database/sql"
)

type SchemaVersion struct {
	ID       int
	Version  int
	Checksum string
}

func (s *SchemaVersion) Equal(o *SchemaVersion) bool {
	if s == o {
		return true
	}

	if s == nil || o == nil {
		return false
	}

	return s.ID == o.ID && s.Version == o.Version && s.Checksum == o.Checksum
}

type LimitedDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Dialect interface {
	CreateVersionTableQuery() string
	CurrentVersionQuery() string
	SaveVersionQuery() string
}

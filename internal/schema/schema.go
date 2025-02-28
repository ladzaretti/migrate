package schema

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ladzaretti/migrate/types"
)

func CreateTable(ctx context.Context, db types.LimitedDB, dialect types.Dialect) error {
	return execContext(ctx, db, dialect.CreateVersionTableQuery())
}

func CurrentVersion(ctx context.Context, db types.LimitedDB, dialect types.Dialect) (types.Schema, error) {
	row := db.QueryRowContext(ctx, dialect.CurrentVersionQuery())

	return scanSchema(row)
}

func SaveVersion(ctx context.Context, db types.LimitedDB, dialect types.Dialect, s types.Schema) error {
	return execContext(ctx, db, dialect.SaveVersionQuery(), s.Version, s.Checksum)
}

func execContext(ctx context.Context, db types.LimitedDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		//nolint:errorlint // errors are not intended to be matched by the user
		return fmt.Errorf("exec context: %v", err)
	}

	return nil
}

func scanSchema(row *sql.Row) (types.Schema, error) {
	ver := types.Schema{}

	if err := row.Scan(&ver.Version, &ver.Checksum); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ver, nil
		}

		//nolint:errorlint // errors are not intended to be matched by the user
		return types.Schema{}, fmt.Errorf("scan schema version: %v", err)
	}

	return ver, nil
}

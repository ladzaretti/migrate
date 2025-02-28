package migratetest

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ladzaretti/migrate/internal/schema"
	"github.com/ladzaretti/migrate/types"
)

func TestDialect(ctx context.Context, db *sql.DB, dialect types.Dialect) error {
	if err := schema.CreateTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("create schema version table: %w", err)
	}

	if _, err := schema.CurrentVersion(ctx, db, dialect); err != nil {
		return fmt.Errorf("fetch current schema version: %w", err)
	}

	sch := types.Schema{
		Version:  100,
		Checksum: "checksum",
	}

	if err := schema.SaveVersion(ctx, db, dialect, sch); err != nil {
		return fmt.Errorf("save schema version: %w", err)
	}

	curr, err := schema.CurrentVersion(ctx, db, dialect)
	if err != nil {
		return fmt.Errorf("fetch updated schema version: %w", err)
	}

	if curr != sch {
		return fmt.Errorf("schema version mismatch: got %+v, expected %+v", curr, sch)
	}

	return nil
}

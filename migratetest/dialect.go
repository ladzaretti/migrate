package migratetest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/ladzaretti/migrate/internal/schemaops"
	"github.com/ladzaretti/migrate/types"
)

func TestDialect(ctx context.Context, db *sql.DB, dialect types.Dialect) error {
	if err := schemaops.CreateTable(ctx, db, dialect); err != nil {
		return fmt.Errorf("create schema version table: %w", err)
	}

	_, err := schemaops.CurrentVersion(ctx, db, dialect)
	if err != nil && !errors.Is(err, schemaops.ErrNoSchemaVersion) {
		return fmt.Errorf("fetch current schema version: %w", err)
	}

	ver1 := types.SchemaVersion{
		ID:       0,
		Version:  1,
		Checksum: "checksum1",
	}

	ver2 := types.SchemaVersion{
		ID:       0,
		Version:  2,
		Checksum: "checksum2",
	}

	if err := schemaops.SaveVersion(ctx, db, dialect, ver1); err != nil {
		return fmt.Errorf("save schema version: %w", err)
	}

	curr, err := schemaops.CurrentVersion(ctx, db, dialect)
	if err != nil {
		return fmt.Errorf("fetch updated schema version: %w", err)
	}

	if curr == nil {
		return errors.New("schema version not found")
	}

	if !curr.Equal(&ver1) {
		return fmt.Errorf("schema version mismatch: got %+v, expected %+v", curr, &ver1)
	}

	if err := schemaops.SaveVersion(ctx, db, dialect, ver2); err != nil {
		return fmt.Errorf("save schema version: %w", err)
	}

	curr, err = schemaops.CurrentVersion(ctx, db, dialect)
	if err != nil {
		return fmt.Errorf("fetch updated schema version: %w", err)
	}

	if curr == nil {
		return errors.New("schema version not found")
	}

	if !curr.Equal(&ver2) {
		return fmt.Errorf("schema version mismatch: got %+v, expected %+v", curr, &ver1)
	}

	return nil
}

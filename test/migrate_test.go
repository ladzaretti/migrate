package test

import (
	"database/sql"
	"testing"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

func createSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

var migrations = []string{
	`
CREATE TABLE
	IF NOT EXISTS testing_migration (
		aid INTEGER PRIMARY KEY,
		another_id INTEGER,
		something_else TEXT
	);
    `,
	`
CREATE TABLE
	IF NOT EXISTS testing_migration_2 (
		id INTEGER PRIMARY KEY,
		another_id INTEGER,
		something_else TEXT
	);
	`,
}

func TestMigrate_Apply(t *testing.T) {
	db := createSQLiteDB(t)

	m := migrate.New(db, migrate.SQLiteDialect{})
	if err := m.Apply(migrations); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}
}

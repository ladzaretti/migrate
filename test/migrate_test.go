package test

import (
	"database/sql"
	"testing"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

var (
	migration01 = `
	CREATE TABLE
		IF NOT EXISTS testing_migration_1 (
			id INTEGER PRIMARY KEY,
			another_id INTEGER,
			something_else TEXT
		);
	    `

	migration02 = `
	CREATE TABLE
		IF NOT EXISTS testing_migration_2 (
			id INTEGER PRIMARY KEY,
			another_id INTEGER,
			something_else TEXT
		);
		`
)

// createSQLiteDB is a testing helper that creates an in-memory sqlite
// database connection.
func createSQLiteDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() { db.Close() })

	return db
}

func TestMigrate_Apply_validMigration(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if err := m.Apply([]string{migration01}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrate_Apply_multiStageMigration(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if err := m.Apply([]string{migration01}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply([]string{migration01, migration02}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("Length of post = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_RollsBackOnError(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if err := m.Apply([]string{migration01}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	err := m.Apply([]string{migration01, migration02, "invalid migration script"})

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("Length of post = %v, want %v", got, want)
	}
}

func currentSchemaVersion(m *migrate.Migration) int {
	currentVersion, err := m.CurrentSchemaVersion()
	if err != nil {
		return -1
	}

	return currentVersion.Version
}

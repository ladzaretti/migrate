package migrate_test

import (
	"context"
	"database/sql"
	"embed"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
	"github.com/ladzaretti/migrate/migratetest"
)

var (
	//go:embed testdata/sqlite/migrations
	embedSQLiteFS embed.FS

	embeddedSQLiteMigrations = migrate.EmbeddedMigrations{
		FS:   embedSQLiteFS,
		Path: "testdata/sqlite/migrations",
	}
)

// createSQLiteDB is a testing helper that creates an in-memory sqlite
// database connection.
func createSQLiteDB(_ context.Context, t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func TestMigrateWithSQLite(t *testing.T) {
	rawMigrations := []string{
		`CREATE TABLE
			IF NOT EXISTS testing_migration_1 (
				id INTEGER PRIMARY KEY,
				another_id INTEGER,
				something_else TEXT
			);
		`,
		`CREATE TABLE
			IF NOT EXISTS testing_migration_2 (
				id INTEGER PRIMARY KEY,
				another_id INTEGER,
				something_else TEXT
			);
		`,
	}

	suite, err := newTestSuite(testSuiteConfig{
		dbHelper:           createSQLiteDB,
		dialect:            migrate.SQLiteDialect{},
		embeddedMigrations: embeddedSQLiteMigrations,
		rawMigrations:      rawMigrations,
	})
	if err != nil {
		t.Fatalf("create test suite: %v", err)
	}

	t.Run("TestDialect", func(t *testing.T) {
		if err := migratetest.TestDialect(t.Context(), suite.dbHelper(t.Context(), t), migrate.SQLiteDialect{}); err != nil {
			t.Fatalf("TestDialect: %v", err)
		}
	})

	t.Run("ApplyStringMigrations", suite.applyStringMigrations)
	t.Run("ApplyEmbeddedMigrations", suite.applyEmbeddedMigrations)
	t.Run("ApplyWithTxDisabled", suite.applyWithTxDisabled)
	t.Run("ApplyWithNoChecksumValidation", suite.applyWithNoChecksumValidation)
	t.Run("ApplyWithFilter", suite.applyWithFilter)
	t.Run("ReapplyAll", suite.reapplyAll)
	t.Run("RollsBackOnSQLError", suite.rollsBackOnSQLError)
	t.Run("RollsBackOnValidationError", suite.rollsBackOnValidationError)
}

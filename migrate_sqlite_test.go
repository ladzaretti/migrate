package migrate_test

import (
	"database/sql"
	"embed"
	"testing"

	"github.com/ladzaretti/migrate"
)

var (
	//go:embed testdata/sqlite/migrations
	embedFS embed.FS

	embeddedMigrations = migrate.EmbeddedMigrations{
		FS:   embedFS,
		Path: "testdata/sqlite/migrations",
	}
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

func TestMigrateWithSQLite(t *testing.T) {
	suite, err := newTestSuite(testSuiteConfig{
		dbHelper:           createSQLiteDB,
		dialect:            migrate.SQLiteDialect{},
		embeddedMigrations: embeddedMigrations,
		stringMigrations:   []string{migration01, migration02},
	})
	if err != nil {
		t.Fatalf("create test suite: %v", err)
	}

	t.Run("ApplyStringMigrations", suite.applyStringMigrations)
	t.Run("ApplyEmbeddedMigrations", suite.applyEmbeddedMigrations)
	t.Run("ApplyWithTxDisabled", suite.applyWithTxDisabled)
	t.Run("RollsBackOnSQLError", suite.rollsBackOnSQLError)
	t.Run("RollsBackOnValidationError", suite.rollsBackOnValidationError)
	t.Run("ApplyWithNoChecksumValidation", suite.applyWithNoChecksumValidation)
	t.Run("ApplyWithFilter", suite.applyWithFilter)
}

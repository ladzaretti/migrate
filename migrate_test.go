package migrate_test

import (
	"database/sql"
	"embed"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

//go:embed migrations
var embeddedMigrations embed.FS

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

func TestMigrate_Apply_stringMigrations(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_embeddedMigrations(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	source := fromEmbeddedSource(embeddedMigrations, "migrations")

	if err := m.Apply(source); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(source); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_withTxDisabled(t *testing.T) {
	db := createSQLiteDB(t)

	opts := []migrate.Opt{
		migrate.WithTransaction(false),
	}
	m := migrate.New(db, migrate.SQLiteDialect{}, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	source := fromEmbeddedSource(embeddedMigrations, "migrations")

	if err := m.Apply(source); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}
}

func TestMigrate_Apply_rollsBackOnSQLError(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	err := m.Apply(fromStringSource(migration01, migration02, "invalid migration script"))

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	gotErr, wantErr := err.Error(), `apply migration script 3: exec context: SQL logic error: near "invalid": syntax error (1)`
	if gotErr != wantErr {
		t.Errorf("error %q, want %q", gotErr, wantErr)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("current version = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_rollsBackOnValidationError(t *testing.T) {
	db := createSQLiteDB(t)
	m := migrate.New(db, migrate.SQLiteDialect{})

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	// run the same migration again
	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	// run corrupted migration
	corruptedMigration02 := migration02 + "this string wasn't here before"
	err := m.Apply(fromStringSource(migration01, corruptedMigration02))

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	gotErr, wantErr := err.Error(), `schema integrity check failed: runtime checksum "77671fcde23b60aff173d65f98bc3863ce38dc83" != database checksum "8165caac3ad7938e2c5aed4f14355fb084b83ef1"`
	if gotErr != wantErr {
		t.Errorf("error %q, want %q", gotErr, wantErr)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("current version = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_withNoChecksumValidation(t *testing.T) {
	db := createSQLiteDB(t)
	opts := []migrate.Opt{
		migrate.WithChecksumValidation(false),
	}
	m := migrate.New(db, migrate.SQLiteDialect{}, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	// run the same migration again
	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	modifiedMigration02 := migration02 + "this string wasn't here before"
	if err := m.Apply(fromStringSource(migration01, modifiedMigration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 2; got != want {
		t.Errorf("current version = %v, want %v", got, want)
	}
}

func TestMigrate_Apply_withFilter(t *testing.T) {
	db := createSQLiteDB(t)
	opts := []migrate.Opt{
		migrate.WithFilter(func(migrationNumber int) bool {
			return migrationNumber != 2
		}),
	}
	m := migrate.New(db, migrate.SQLiteDialect{}, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version %d, got %d", got, want)
	}

	if err := m.Apply(fromStringSource(migration01, migration02)); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("expected schema version = %v, want %v", got, want)
	}
}

func fromEmbeddedSource(fs embed.FS, p string) migrate.EmbeddedMigrations {
	return migrate.EmbeddedMigrations{FS: fs, Path: p}
}

func fromStringSource(s ...string) migrate.StringMigrations {
	return migrate.StringMigrations(s)
}

func currentSchemaVersion(m *migrate.Migrator) int {
	currentVersion, err := m.CurrentSchemaVersion()
	if err != nil {
		return -1
	}

	return currentVersion.Version
}

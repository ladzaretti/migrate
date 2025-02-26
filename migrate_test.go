package migrate_test

import (
	"database/sql"
	"errors"
	"fmt"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

type testSuiteConfig struct {
	dbHelper           func(t *testing.T) *sql.DB
	dialect            migrate.DialectAdapter
	embeddedMigrations migrate.EmbeddedMigrations
	stringMigrations   []string
}

type testSuite struct {
	testSuiteConfig
}

// TODO1: pg test

func newTestSuite(c testSuiteConfig) (*testSuite, error) {
	if len(c.stringMigrations) < 2 {
		return nil, errors.New("stringMigrations must have at least 2 elements")
	}

	embeddedMigrations, err := c.embeddedMigrations.List()
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}

	if len(embeddedMigrations) < 2 {
		return nil, errors.New("embeddedMigrations must have at least 2 elements")
	}

	return &testSuite{testSuiteConfig: c}, nil
}

func (s *testSuite) applyStringMigrations(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations[0])); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("expected schema version = %v, want %v", got, want)
	}
}

func (s *testSuite) applyEmbeddedMigrations(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	migrations, _ := s.embeddedMigrations.List()

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(s.embeddedMigrations); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(migrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(s.embeddedMigrations); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(migrations); got != want {
		t.Errorf("expected schema version = %v, want %v", got, want)
	}
}

func (s *testSuite) applyWithTxDisabled(t *testing.T) {
	db := s.dbHelper(t)

	opts := []migrate.Opt{
		migrate.WithTransaction(false),
	}
	m := migrate.New(db, s.dialect, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(s.embeddedMigrations); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	migrations, _ := s.embeddedMigrations.List()
	if got, want := currentSchemaVersion(m), len(migrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) rollsBackOnSQLError(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations[0])); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	migrations := copyAppend(s.stringMigrations, "invalid migration script")
	err := m.Apply(fromStringSource(migrations...))

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	gotErr, wantErr := err.Error(), `apply migration script 3: exec context: SQL logic error: near "invalid": syntax error (1)`
	if gotErr != wantErr {
		t.Errorf("unexpected error: got %q, want %q", gotErr, wantErr)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) rollsBackOnValidationError(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run the same migration again
	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	corrupted := copyAppend(s.stringMigrations)
	corrupted[len(corrupted)-1] += "this string wasn't here before"

	// run corrupted migration
	err := m.Apply(fromStringSource(corrupted...))

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	gotErr, wantErr := err.Error(), `schema integrity check failed: runtime checksum "77671fcde23b60aff173d65f98bc3863ce38dc83" != database checksum "8165caac3ad7938e2c5aed4f14355fb084b83ef1"`
	if gotErr != wantErr {
		t.Errorf("unexpected error: got %q, want %q", gotErr, wantErr)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) applyWithNoChecksumValidation(t *testing.T) {
	db := s.dbHelper(t)
	opts := []migrate.Opt{
		migrate.WithChecksumValidation(false),
	}
	m := migrate.New(db, s.dialect, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run the same migration again
	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	corrupted := copyAppend(s.stringMigrations)
	corrupted[len(corrupted)-1] += "this string wasn't here before"

	if err := m.Apply(fromStringSource(corrupted...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) applyWithFilter(t *testing.T) {
	db := createSQLiteDB(t)
	opts := []migrate.Opt{
		migrate.WithFilter(func(migrationNumber int) bool {
			return migrationNumber != 1
		}),
	}
	m := migrate.New(db, migrate.SQLiteDialect{}, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	if err := m.Apply(fromStringSource(s.stringMigrations[0])); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 0; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	opts = []migrate.Opt{
		migrate.WithFilter(func(migrationNumber int) bool {
			return migrationNumber != 2
		}),
	}

	m = migrate.New(db, migrate.SQLiteDialect{}, opts...)
	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	m = migrate.New(db, migrate.SQLiteDialect{})
	if err := m.Apply(fromStringSource(s.stringMigrations...)); err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := currentSchemaVersion(m), len(s.stringMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
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

func copyAppend[T any](s []T, el ...T) []T {
	cs := make([]T, len(s), len(s)+len(el))
	copy(cs, s)

	return append(cs, el...)
}

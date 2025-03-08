package migrate_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ladzaretti/migrate"
	"github.com/ladzaretti/migrate/types"
)

type testSuiteConfig struct {
	dbHelper           func(*testing.T) *sql.DB
	dialect            types.Dialect
	embeddedMigrations migrate.EmbeddedMigrations
	rawMigrations      []string
}

type testSuite struct {
	testSuiteConfig
}

func newTestSuite(conf testSuiteConfig) (*testSuite, error) {
	if len(conf.rawMigrations) < 2 {
		return nil, errors.New("stringMigrations must have at least 2 elements")
	}

	embeddedMigrations, err := conf.embeddedMigrations.List()
	if err != nil {
		return nil, fmt.Errorf("list embedded migrations: %w", err)
	}

	if len(embeddedMigrations) < 2 {
		return nil, errors.New("embeddedMigrations must have at least 2 elements")
	}

	return &testSuite{testSuiteConfig: conf}, nil
}

func (s *testSuite) applyStringMigrations(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations[0]))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 1; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}
	if got, want := n, len(s.rawMigrations)-1; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
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

	n, err := m.Apply(s.embeddedMigrations)
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(migrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(migrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err = m.Apply(s.embeddedMigrations)
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
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

	migrations, _ := s.embeddedMigrations.List()

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err := m.Apply(s.embeddedMigrations)
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(migrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(migrations); got != want {
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

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(s.rawMigrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run the same migration again
	//

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run corrupted migration
	//

	corrupted := copyAppend(s.rawMigrations)
	corrupted[len(corrupted)-1] += "this string wasn't here before"

	n, err = m.Apply(stringMigrationsFrom(corrupted...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) applyWithFilter(t *testing.T) {
	db := s.dbHelper(t)
	opts := []migrate.Opt{
		migrate.WithFilter(func(migrationNumber int) bool {
			return migrationNumber != 1
		}),
	}
	m := migrate.New(db, s.dialect, opts...)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations[0]))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), 0; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	opts = []migrate.Opt{
		migrate.WithFilter(func(migrationNumber int) bool {
			return migrationNumber != 2
		}),
	}
	m = migrate.New(db, s.dialect, opts...)

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 1; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	m = migrate.New(db, s.dialect)

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(s.rawMigrations)-1; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) reapplyAll(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(s.rawMigrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	opts := []migrate.Opt{
		migrate.WithReapplyAll(true),
	}
	m = migrate.New(db, s.dialect, opts...)

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(s.rawMigrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func (s *testSuite) rollsBackOnSQLError(t *testing.T) {
	db := s.dbHelper(t)
	m := migrate.New(db, s.dialect)

	if got, want := currentSchemaVersion(m), -1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations[0]))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}
	if got, want := n, 1; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), 1; got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run corrupted migration
	//

	corrupted := copyAppend(s.rawMigrations, "invalid migration script")

	n, err = m.Apply(stringMigrationsFrom(corrupted...))
	if err == nil {
		t.Errorf("expected an error but got none")
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	gotErr, wantPrefix := err.Error(), `apply migration script 3: exec context:`
	if !strings.HasPrefix(gotErr, wantPrefix) {
		t.Errorf("unexpected error: got %q, want prefix %q", gotErr, wantPrefix)
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

	n, err := m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, len(s.rawMigrations); got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	// run the same migration again
	//

	n, err = m.Apply(stringMigrationsFrom(s.rawMigrations...))
	if err != nil {
		t.Errorf("m.Apply() returned an error: %v", err)
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}

	corrupted := copyAppend(s.rawMigrations)
	corrupted[len(corrupted)-1] += "this string wasn't here before"

	// run corrupted migration
	//

	n, err = m.Apply(stringMigrationsFrom(corrupted...))

	if err == nil {
		t.Errorf("expected an error but got none")
	}

	if got, want := n, 0; got != want {
		t.Errorf("applied migrations: got %d, want %d", got, want)
	}

	gotErr, wantPrefix := err.Error(), `schema integrity check failed:`
	if !strings.HasPrefix(gotErr, wantPrefix) {
		t.Errorf("unexpected error: got %q, want prefix %q", gotErr, wantPrefix)
	}

	if got, want := currentSchemaVersion(m), len(s.rawMigrations); got != want {
		t.Errorf("schema version mismatch: got %v, want %v", got, want)
	}
}

func stringMigrationsFrom(s ...string) migrate.StringMigrations {
	return migrate.StringMigrations(s)
}

func currentSchemaVersion(m *migrate.Migrator) int {
	v, err := m.CurrentSchemaVersion(context.Background())
	if err != nil {
		return -1
	}

	return v.Version
}

func copyAppend[T any](s []T, el ...T) []T {
	cs := make([]T, len(s), len(s)+len(el))
	copy(cs, s)

	return append(cs, el...)
}

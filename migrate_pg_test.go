package migrate_test

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ladzaretti/migrate"
	"github.com/ladzaretti/migrate/migratetest"
)

var (
	//go:embed testdata/pg/migrations
	embedPostgresFS            embed.FS
	embeddedPostgresMigrations = migrate.EmbeddedMigrations{
		FS:   embedPostgresFS,
		Path: "testdata/pg/migrations",
	}
)

func postgresTestContainer(ctx context.Context) (*postgres.PostgresContainer, error) {
	ctr, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("database"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("postgres"),
		postgres.WithSQLDriver("pgx"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, fmt.Errorf("create test container: %v", err)
	}

	if err := ctr.Snapshot(ctx); err != nil {
		return nil, fmt.Errorf("create snapshot: %v", err)
	}

	return ctr, nil
}

func setupPostgresTestSuite(ctx context.Context, t *testing.T, stringMigrations []string, embeddedMigrations migrate.EmbeddedMigrations) (*testSuite, func()) {
	t.Helper()

	ctr, err := postgresTestContainer(ctx)
	if err != nil {
		t.Fatalf("create test container: %v", err)
	}

	connString, err := ctr.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}
	cleanup := func() {
		_ = testcontainers.TerminateContainer(ctr)
	}

	helper := func(t *testing.T) *sql.DB {
		t.Helper()

		if err := ctr.Restore(context.Background()); err != nil {
			t.Fatalf("restore database: %v", err)
		}

		db, err := sql.Open("pgx", connString)
		if err != nil {
			t.Fatalf("open database: %v", err)
		}

		t.Cleanup(func() {
			db.Close()
		})

		return db
	}

	suite, err := newTestSuite(testSuiteConfig{
		dbHelper:           helper,
		dialect:            migrate.PostgreSQLDialect{},
		embeddedMigrations: embeddedMigrations,
		stringMigrations:   stringMigrations,
	})
	if err != nil {
		t.Fatalf("create test suite: %v", err)
	}

	return suite, cleanup
}

func TestMigrateWithPostgres(t *testing.T) {
	stringMigrations := []string{
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

	suite, cleanup := setupPostgresTestSuite(context.Background(), t, stringMigrations, embeddedPostgresMigrations)
	defer cleanup()

	t.Run("TestDialect", func(t *testing.T) {
		if err := migratetest.TestDialect(t.Context(), suite.dbHelper(t), migrate.PostgreSQLDialect{}); err != nil {
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

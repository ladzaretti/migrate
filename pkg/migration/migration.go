package migration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type Fingerprint string

type FingerprintFunc func(query string) Fingerprint

type Version struct {
	ID int
	FP Fingerprint
}

type LimitedTx interface {
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
	PrepareContext(ctx context.Context, query string) (*sql.Stmt, error)
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	Stmt(stmt *sql.Stmt) *sql.Stmt
}

type Versioning interface {
	CreateVersionTable(ctx context.Context, tx LimitedTx) error
	LoadVersion(ctx context.Context, tx LimitedTx) (Version, error)
	StoreVersion(ctx context.Context, tx LimitedTx, v Version) error
}

type Migration struct {
	db          *sql.DB
	versioning  Versioning
	fingerprint FingerprintFunc
}

type Opt func(*Migration)

func New(db *sql.DB, opts ...Opt) *Migration {
	m := &Migration{
		db:          db,
		versioning:  DefaultVersioning{},
		fingerprint: nil,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func WithFingerprintFunc(fp FingerprintFunc) Opt {
	return func(m *Migration) {
		m.fingerprint = fp
	}
}

func WithCustomVersioning(v Versioning) Opt {
	return func(m *Migration) {
		m.versioning = v
	}
}

func (m *Migration) Migrate(ctx context.Context, migrations []string) error {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		// FIXME: unify error handling for package
		return fmt.Errorf("failed to start db transaction: %w", err)
	}

	if err := m.versioning.CreateVersionTable(ctx, tx); err != nil {
		return fmt.Errorf("failed to prepare migration environment: %w", err)
	}

	for i, query := range migrations {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return fmt.Errorf("failed to roll back after failed migration %d attempt: %w", i, errors.Join(err, err2))
			}

			return fmt.Errorf("successfully rolled back after migration %d failure: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit the migration transaction: %w", err)
	}

	return nil
}

var _ Versioning = (*DefaultVersioning)(nil)

type DefaultVersioning struct{}

func (v DefaultVersioning) CreateVersionTable(ctx context.Context, tx LimitedTx) error {
	query := `
		CREATE TABLE
		IF NOT EXISTS migration_version (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			hash TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := tx.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("create version table failed: %w", err)
	}

	return nil
}

//nolint:revive // WIP
func (v DefaultVersioning) LoadVersion(ctx context.Context, tx LimitedTx) (Version, error) {
	panic("impl")
}

//nolint:revive // WIP
func (v DefaultVersioning) StoreVersion(ctx context.Context, tx LimitedTx, version Version) error {
	panic("impl")
}

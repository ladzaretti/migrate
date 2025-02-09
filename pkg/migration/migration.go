package migration

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// SignatureFunc is a function type that generates a unique identifier
// for a given input string.
//
// This function is used for schema comparison and versioning.
type SignatureFunc func(s string) string

type Schema struct {
	Version int
	Hash    string
}

type LimitedDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type SchemaVersioning interface {
	CreateSchemaTable(ctx context.Context, db LimitedDB) error
	CurrentSchema(ctx context.Context, db LimitedDB) (Schema, error)
	SaveSchema(ctx context.Context, db LimitedDB, v Schema) error
	GetSchemaFrom(row *sql.Row) (Schema, error)
}

type Migration struct {
	db            *sql.DB
	versioning    SchemaVersioning
	signatureFunc SignatureFunc
}

type Opt func(*Migration)

func New(db *sql.DB, opts ...Opt) *Migration {
	m := &Migration{
		db:            db,
		versioning:    DefaultVersioning{},
		signatureFunc: normalizedSha256,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func WithSignatureFunc(fn SignatureFunc) Opt {
	return func(m *Migration) {
		m.signatureFunc = fn
	}
}

func WithCustomVersioning(v SchemaVersioning) Opt {
	return func(m *Migration) {
		m.versioning = v
	}
}

// TODO: WithValidation ?

func errf(format string, a ...any) error {
	//nolint:err113 // all package errors essentially mean migration failure.
	return fmt.Errorf("migration error: "+format, a...)
}

func (m *Migration) Apply(migrations []string) error {
	return m.ApplyContext(context.Background(), migrations)
}

func (m *Migration) ApplyContext(ctx context.Context, migrations []string) error {
	if err := m.versioning.CreateSchemaTable(ctx, m.db); err != nil {
		return errf("create schema version table error: %v", err)
	}

	migrationsLen := len(migrations)
	current, err := m.versioning.CurrentSchema(ctx, m.db)
	if err != nil {
		return errf("load version error: %v", err)
	}

	if current.Version > migrationsLen {
		return errf("database version is greater than the number of migration scripts provided")
	}

	// TODO: validate schema hash

	if current.Version == migrationsLen {
		return nil // schema is up to date; nothing to do.
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errf("transaction start error: %v", err)
	}

	for i := current.Version; i < migrationsLen; i++ {
		if err := m.applyMigration(ctx, tx, migrations[i]); err != nil {
			if err2 := tx.Rollback(); err2 != nil {
				return fmt.Errorf("failed to roll back after failed migration %d attempt: %w", i, errors.Join(err2, err))
			}

			return fmt.Errorf("successfully rolled back after migration %d failure: %w", i, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return errf("transaction commit error: %v", err)
	}

	return nil
}

func (m *Migration) applyMigration(ctx context.Context, tx LimitedDB, migration string) error {
	if _, err := tx.ExecContext(ctx, migration); err != nil {
		return err
	}

	sch := Schema{
		Hash: m.signatureFunc(migration),
	}

	if err := m.versioning.SaveSchema(ctx, tx, sch); err != nil {
		return fmt.Errorf(": %v", err)
	}

	return nil
}

func (m *Migration) hash(query string) string {
	if m.signatureFunc == nil {
		return ""
	}

	return m.signatureFunc(query)
}

var _ SchemaVersioning = DefaultVersioning{}

type DefaultVersioning struct{}

func (v DefaultVersioning) CreateSchemaTable(ctx context.Context, db LimitedDB) error {
	query := `
		CREATE TABLE
		IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY,
			hash TEXT NOT NULL,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	if _, err := db.ExecContext(ctx, query); err != nil {
		return err
	}

	return nil
}

func (v DefaultVersioning) CurrentSchema(ctx context.Context, db LimitedDB) (Schema, error) {
	query := `
		SELECT version, hash FROM schema_version ORDER BY version DESC LIMIT 1;
	`

	row := db.QueryRowContext(ctx, query)
	ver, err := v.GetSchemaFrom(row)
	if err != nil {
		return Schema{}, err
	}

	const countStar = `
		SELECT count(*) FROM schema_version
	`

	var count int
	if err := db.QueryRowContext(ctx, countStar).Scan(&count); err != nil {
		return Schema{}, err
	}

	if count != ver.Version {
		return Schema{}, fmt.Errorf("schema version table integrity check failed: expected %d entries, found %d", ver.Version, count)
	}

	return ver, nil
}

func (v DefaultVersioning) GetSchemaFrom(row *sql.Row) (Schema, error) {
	ver := Schema{}

	if err := row.Scan(&ver.Version, &ver.Hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ver, nil
		}

		return Schema{}, err
	}

	return ver, nil
}

func (v DefaultVersioning) SaveSchema(ctx context.Context, db LimitedDB, s Schema) error {
	query := `
		INSERT INTO schema_version (hash) VALUES ($1) 
	`

	if _, err := db.ExecContext(ctx, query, s.Hash); err != nil {
		return err
	}

	return nil
}

func normalizedSha256(query string) string {
	normalized := normalize(query)
	hash := sha256.Sum256([]byte(normalized))

	return hex.EncodeToString(hash[:])
}

func normalize(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1 // Remove whitespace
		}

		return r
	}, s)
}

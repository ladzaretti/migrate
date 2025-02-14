//
// This is free and unencumbered software released into the public domain.
//
// Anyone is free to copy, modify, publish, use, compile, sell, or
// distribute this software, either in source code form or as a compiled
// binary, for any purpose, commercial or non-commercial, and by any
// means.
//
// In jurisdictions that recognize copyright laws, the author or authors
// of this software dedicate any and all copyright interest in the
// software to the public domain. We make this dedication for the benefit
// of the public at large and to the detriment of our heirs and
// successors. We intend this dedication to be an overt act of
// relinquishment in perpetuity of all present and future rights to this
// software under copyright law.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
// EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
// MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
// IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
// OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
// ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
// OTHER DEALINGS IN THE SOFTWARE.
//
// For more information, please refer to <https://unlicense.org/>

package migrate

import (
	"context"
	//nolint:gosec
	// SHA-1 is used here for change detection,
	// not for cryptographic security.
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Signer generates a unique identifier for a given input string,
// used to sign migrations.
//
// It is used for schema comparison and validation.
type Signer func(s string) string

type Filter func(index int) bool

type LimitedDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Schema struct {
	Version int
	Hash    string
}

type Migration struct {
	db        *sql.DB
	dialect   DialectAdapter
	sign      Signer
	filter    Filter
	disableTx bool
}

type Opt func(*Migration)

func New(db *sql.DB, dialect DialectAdapter, opts ...Opt) *Migration {
	m := &Migration{
		db:      db,
		sign:    normalizedSha1,
		filter:  func(_ int) bool { return true },
		dialect: SQLiteDialect{},
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func WithCustomSigning(fn Signer) Opt {
	return func(m *Migration) {
		m.sign = fn
	}
}

func WithTransactionDisabled(disabled bool) Opt {
	return func(m *Migration) {
		m.disableTx = disabled
	}
}

func WithFilter(fn Filter) Opt {
	return func(m *Migration) {
		m.filter = fn
	}
}

type ApplyError struct {
	Index int
	Err   error
}

func (e *ApplyError) Error() string {
	return fmt.Sprintf("failed to apply migration %d: %v", e.Index, e.Err)
}

func (e *ApplyError) Unwrap() error {
	return e.Err
}

func errf(format string, a ...any) error {
	//nolint:err113 // all package errors essentially mean migration failure.
	return fmt.Errorf("migration error: "+format, a...)
}

func (m *Migration) Apply(migrations []string) error {
	return m.ApplyContext(context.Background(), migrations)
}

func (m *Migration) ApplyContext(ctx context.Context, migrations []string) error {
	if err := createSchemaVersionTable(ctx, m.db, m.dialect); err != nil {
		return errf("create schema version table error: %v", err)
	}

	schema, err := currentSchemaVersion(ctx, m.db, m.dialect)
	if err != nil {
		return errf("load version error: %v", err)
	}

	if schema.Version > len(migrations) {
		return errf("database version (%d) exceeds available migrations (%d)", schema.Version, len(migrations))
	}

	hashHistory := m.hashHistory(migrations)
	if schema.Version > 0 && schema.Hash != hashHistory[schema.Version] {
		return errf("schema integrity check failed: expected hash %q, got %q", hashHistory[schema.Version], schema.Hash)
	}

	if schema.Version == len(migrations) {
		return nil // already up to date
	}

	if m.disableTx {
		if err := m.applyMigrations(ctx, m.db, schema.Version, migrations, hashHistory); err != nil {
			return errf("non-transactional migration failed: %w", err)
		}

		return nil
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errf("start transaction error: %v", err)
	}

	if err := m.applyMigrations(ctx, tx, schema.Version, migrations, hashHistory); err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return errf("rollback failed: %v", errors.Join(err2, err))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return errf("transaction commit error: %v", err)
	}

	return nil
}

func (m *Migration) applyMigrations(ctx context.Context, db LimitedDB, current int, migrations []string, hashes []string) error {
	for i := current; i < len(migrations); i++ {
		if !m.filter(i) {
			continue
		}

		sch := Schema{Version: i + 1, Hash: hashes[i+1]}
		if err := applyMigration(ctx, db, m.dialect, sch, migrations[i]); err != nil {
			return &ApplyError{Index: i, Err: err}
		}
	}

	return nil
}

func (m *Migration) hashHistory(migrations []string) []string {
	history := make([]string, len(migrations)+1)

	history[0] = "" // Version 0 has no migrations applied

	for i := 1; i <= len(migrations); i++ {
		history[i] = m.sign(history[i-1] + m.sign(migrations[i-1]))
	}

	return history
}

func applyMigration(ctx context.Context, db LimitedDB, dia DialectAdapter, sch Schema, migration string) error {
	if err := execContext(ctx, db, migration); err != nil {
		return err
	}

	if err := saveSchemaVersion(ctx, db, dia, sch); err != nil {
		return err
	}

	return nil
}

func createSchemaVersionTable(ctx context.Context, db LimitedDB, dialect DialectAdapter) error {
	return execContext(ctx, db, dialect.CreateVersionTableQuery())
}

func saveSchemaVersion(ctx context.Context, db LimitedDB, dialect DialectAdapter, s Schema) error {
	return execContext(ctx, db, dialect.SaveVersionQuery(), s.Version, s.Hash)
}

func currentSchemaVersion(ctx context.Context, db LimitedDB, dialect DialectAdapter) (Schema, error) {
	row := db.QueryRowContext(ctx, dialect.CurrentVersionQuery())

	return scanSchema(row)
}

func execContext(ctx context.Context, db LimitedDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("exec context failed: %w", err)
	}

	return nil
}

func scanSchema(row *sql.Row) (Schema, error) {
	ver := Schema{}

	if err := row.Scan(&ver.Version, &ver.Hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ver, nil
		}

		return Schema{}, fmt.Errorf("failed to scan schema version: %w", err)
	}

	return ver, nil
}

func normalizedSha1(query string) string {
	normalized := normalize(query)
	//nolint:gosec
	// SHA-1 is used here for change detection,
	// not for cryptographic security.
	hash := sha1.Sum([]byte(normalized))

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

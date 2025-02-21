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
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// Signer generates a typically unique identifier for a given input string,
// used to sign migrations.
//
// It is used for schema comparison and validation.
type Signer func(s string) string

type Filter func(migrationNumber int) bool

type LimitedDB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type Schema struct {
	Version  int
	Checksum string
}

type Migration struct {
	db                     *sql.DB
	dialect                DialectAdapter
	sign                   Signer
	migrationFilter        Filter
	withTx                 bool
	withChecksumValidation bool
}

type Opt func(*Migration)

func New(db *sql.DB, dialect DialectAdapter, opts ...Opt) *Migration {
	m := &Migration{
		db:                     db,
		dialect:                SQLiteDialect{},
		sign:                   normalizedSha1,
		migrationFilter:        func(_ int) bool { return true },
		withTx:                 true,
		withChecksumValidation: true,
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

func WithTransactions(enabled bool) Opt {
	return func(m *Migration) {
		m.withTx = enabled
	}
}

func WithChecksumValidation(enabled bool) Opt {
	return func(m *Migration) {
		m.withChecksumValidation = enabled
	}
}

func WithFilter(fn Filter) Opt {
	return func(m *Migration) {
		m.migrationFilter = fn
	}
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func (m *Migration) Apply(migrations []string) error {
	return m.ApplyContext(context.Background(), migrations)
}

func (m *Migration) ApplyContext(ctx context.Context, migrations []string) error {
	if err := createSchemaVersionTable(ctx, m.db, m.dialect); err != nil {
		return errf("create schema version table: %v", err)
	}

	schema, err := currentSchemaVersion(ctx, m.db, m.dialect)
	if err != nil {
		return errf("current schema version: %v", err)
	}

	if schema.Version > len(migrations) {
		return errf("database version (%d) exceeds available migrations (%d)", schema.Version, len(migrations))
	}

	runtimeChecksum := m.checksumHistory(migrations)
	if err := m.validateChecksum(schema, runtimeChecksum); err != nil {
		return errf("schema integrity check failed: %v", err)
	}

	if schema.Version == len(migrations) {
		return nil // already up to date
	}

	if !m.withTx {
		if err := m.applyMigrations(ctx, m.db, schema.Version, migrations, runtimeChecksum); err != nil {
			return errf("non-transactional migration: %w", err)
		}

		return nil
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return errf("start transaction: %v", err)
	}

	if err := m.applyMigrations(ctx, tx, schema.Version, migrations, runtimeChecksum); err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return errf("rollback: %v", errors.Join(err2, err))
		}

		return err
	}

	if err := tx.Commit(); err != nil {
		return errf("transaction commit: %v", err)
	}

	return nil
}

func (m *Migration) CurrentSchemaVersion() (Schema, error) {
	return currentSchemaVersion(context.Background(), m.db, m.dialect)
}

func (m *Migration) applyMigrations(ctx context.Context, db LimitedDB, current int, migrations []string, checksums []string) error {
	for i := current; i < len(migrations); i++ {
		if !m.migrationFilter(i + 1) {
			continue
		}

		sch := Schema{Version: i + 1, Checksum: checksums[i+1]}
		if err := applyMigration(ctx, db, m.dialect, sch, migrations[i]); err != nil {
			return errf("apply migration script %d: %v", i+1, err)
		}
	}

	return nil
}

func (m *Migration) checksumHistory(migrations []string) []string {
	history := make([]string, len(migrations)+1)
	history[0] = "" // version 0 has no migrations applied

	for i := 1; i <= len(migrations); i++ {
		history[i] = m.sign(history[i-1] + m.sign(migrations[i-1]))
	}

	return history
}

func (m *Migration) validateChecksum(dbSchema Schema, runtimeChecksum []string) error {
	if !m.withChecksumValidation {
		return nil
	}

	if dbSchema.Version == 0 {
		return nil
	}

	if dbSchema.Checksum != runtimeChecksum[dbSchema.Version] {
		return errf("runtime checksum %q != database checksum %q", runtimeChecksum[dbSchema.Version], dbSchema.Checksum)
	}

	return nil
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
	return execContext(ctx, db, dialect.SaveVersionQuery(), s.Version, s.Checksum)
}

func currentSchemaVersion(ctx context.Context, db LimitedDB, dialect DialectAdapter) (Schema, error) {
	row := db.QueryRowContext(ctx, dialect.CurrentVersionQuery())

	return scanSchema(row)
}

func execContext(ctx context.Context, db LimitedDB, query string, args ...any) error {
	if _, err := db.ExecContext(ctx, query, args...); err != nil {
		//nolint:errorlint // errors are not intended to be matched by the user
		return fmt.Errorf("exec context: %v", err)
	}

	return nil
}

func scanSchema(row *sql.Row) (Schema, error) {
	ver := Schema{}

	if err := row.Scan(&ver.Version, &ver.Checksum); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ver, nil
		}

		return Schema{}, fmt.Errorf("scan schema version: %w", err)
	}

	return ver, nil
}

func normalizedSha1(query string) string {
	normalized := normalize(query)
	//nolint:gosec // in this context, SHA-1 is for change detection, not security.
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

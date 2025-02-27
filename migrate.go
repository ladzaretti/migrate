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

// Checksum is a function type that generates a unique checksum for the input string.
// It is used for schema validation and comparison in migrations.
type Checksum func(s string) string

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

type Migrator struct {
	db                     *sql.DB
	dialect                DialectAdapter
	migrationFilter        Filter
	checksum               Checksum
	withChecksumValidation bool
	withTx                 bool
	reapplyAll             bool
}

type Opt func(*Migrator)

func New(db *sql.DB, dialect DialectAdapter, opts ...Opt) *Migrator {
	m := &Migrator{
		db:                     db,
		dialect:                dialect,
		migrationFilter:        func(_ int) bool { return true },
		checksum:               normalizedSha1,
		withChecksumValidation: true,
		withTx:                 true,
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

func WithChecksum(fn Checksum) Opt {
	return func(m *Migrator) {
		m.checksum = fn
	}
}

func WithTransaction(enabled bool) Opt {
	return func(m *Migrator) {
		m.withTx = enabled
	}
}

func WithChecksumValidation(enabled bool) Opt {
	return func(m *Migrator) {
		m.withChecksumValidation = enabled
	}
}

func WithFilter(fn Filter) Opt {
	return func(m *Migrator) {
		m.migrationFilter = fn
	}
}

func WithReapplyAll(enabled bool) Opt {
	return func(m *Migrator) {
		m.reapplyAll = enabled
	}
}

func errf(format string, a ...any) error {
	return fmt.Errorf(format, a...)
}

func (m *Migrator) Apply(from Source) (int, error) {
	return m.ApplyContext(context.Background(), from)
}

func (m *Migrator) ApplyContext(ctx context.Context, from Source) (int, error) {
	migrations, err := from.List()
	if err != nil {
		return 0, errf("list migrations source: %v", err)
	}

	if err := createSchemaVersionTable(ctx, m.db, m.dialect); err != nil {
		return 0, errf("create schema version table: %v", err)
	}

	schema, err := currentSchemaVersion(ctx, m.db, m.dialect)
	if err != nil {
		return 0, errf("current schema version: %v", err)
	}

	if schema.Version > len(migrations) {
		return 0, errf("database version (%d) exceeds available migrations (%d)", schema.Version, len(migrations))
	}

	runtimeChecksum := m.checksumHistory(migrations)
	if err := m.validateChecksum(schema, runtimeChecksum); err != nil {
		return 0, errf("schema integrity check failed: %v", err)
	}

	if schema.Version == len(migrations) {
		return 0, nil // already up to date
	}

	if !m.withTx {
		n, err := m.applyMigrations(ctx, m.db, schema.Version, migrations, runtimeChecksum)
		if err != nil {
			return n, errf("non-transactional migration: %w", err)
		}

		return n, err
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return 0, errf("start transaction: %v", err)
	}

	n, err := m.applyMigrations(ctx, tx, schema.Version, migrations, runtimeChecksum)
	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return 0, errf("rollback: %v", errors.Join(err2, err))
		}

		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, errf("transaction commit: %v", err)
	}

	return n, err
}

func (m *Migrator) CurrentSchemaVersion() (Schema, error) {
	return currentSchemaVersion(context.Background(), m.db, m.dialect)
}

func (m *Migrator) applyMigrations(ctx context.Context, db LimitedDB, current int, migrations []string, checksums []string) (n int, retErr error) {
	if len(migrations)+1 != len(checksums) {
		retErr = errf("mismatched migrations and checksums: expected %d checksums (+1 for initial state), but found %d", len(migrations), len(checksums))
		return
	}

	from := current
	if m.reapplyAll {
		from = 0
	}

	for i := from; i < len(migrations); i++ {
		if !m.migrationFilter(i + 1) {
			continue
		}

		sch := Schema{Version: i + 1, Checksum: checksums[i+1]}
		if err := applyMigration(ctx, db, m.dialect, sch, migrations[i]); err != nil {
			retErr = errf("apply migration script %d: %v", i+1, err)
			return
		}

		n++
	}

	return
}

func (m *Migrator) checksumHistory(migrations []string) []string {
	history := make([]string, len(migrations)+1)
	history[0] = "" // version 0 has no migrations applied

	for i, mig := range migrations {
		history[i+1] = m.checksum(history[i] + m.checksum(mig))
	}

	return history
}

func (m *Migrator) validateChecksum(dbSchema Schema, runtimeChecksum []string) error {
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

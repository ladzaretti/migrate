package test

import (
	"database/sql"
	"testing"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
	"github.com/ladzaretti/migrate/test/migrations"
)

func TestMigrate_New(t *testing.T) {
	db, err := sql.Open("sqlite", "/tmp/.sqlite")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	migrations := migrations.Scripts()

	m := migrate.New(db, migrate.SQLiteDialect{})

	if err := m.Apply(migrations); err != nil {
		t.Fatalf("Failed to migrate database: %v", err)
	}

	var timestamp string
	if err := db.QueryRow("select CURRENT_TIMESTAMP").Scan(&timestamp); err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if timestamp == "" {
		t.Fatalf("timestamp should not be empty")
	}
}

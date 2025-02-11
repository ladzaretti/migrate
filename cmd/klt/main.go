package main

import (
	"database/sql"
	"log"

	_ "embed"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/keelite/migrations"
	"github.com/ladzaretti/keelite/pkg/migration"
)

func main() {
	db, err := sql.Open("sqlite", "/tmp/.sqlite")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}

	migrations := migrations.Scripts()

	_ = migrations

	m := migration.New(db, migration.SQLiteDialect{})
	if err := m.Apply(migrations); err != nil {
		log.Fatalf("Failed to migrate database: %v", err)
	}

	var timestamp string

	if err := db.QueryRow("select CURRENT_TIMESTAMP").Scan(&timestamp); err != nil {
		log.Fatalf("query failed")
	}

	_ = timestamp
}

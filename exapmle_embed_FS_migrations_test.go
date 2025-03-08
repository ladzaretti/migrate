package migrate_test

import (
	"database/sql"
	"embed"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

var (
	//go:embed testdata/sqlite/migrations
	embedFS embed.FS

	embeddedMigrations = migrate.EmbeddedMigrations{
		FS:   embedFS,
		Path: "testdata/sqlite/migrations",
	}
)

// Apply migrations embedded in the binary using embed.FS.
func Example_applyEmbedFSMigrations() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		fmt.Printf("open: %v", err)
		return
	}
	defer db.Close()

	m := migrate.New(db, migrate.SQLiteDialect{})

	n, err := m.Apply(embeddedMigrations)
	if err != nil {
		fmt.Printf("migration apply: %v", err)
		return
	}

	fmt.Printf("applied migrations: %d", n)
	// Output: applied migrations: 2
}

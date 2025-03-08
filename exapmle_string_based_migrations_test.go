package migrate_test

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

var scripts = []string{
	"CREATE TABLE foo (id INTEGER PRIMARY KEY);",
	"CREATE TABLE bar (id INTEGER PRIMARY KEY);",
}

// Apply migrations directly from strings without using external files.
func Example_applyStringBasedMigrations() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		fmt.Printf("open: %v", err)
		return
	}
	defer db.Close()

	m := migrate.New(db, migrate.SQLiteDialect{})

	n, err := m.Apply(migrate.StringMigrations(scripts))
	if err != nil {
		fmt.Printf("migration apply: %v", err)
		return
	}

	fmt.Printf("applied migrations: %d", n)
	// Output: applied migrations: 2
}

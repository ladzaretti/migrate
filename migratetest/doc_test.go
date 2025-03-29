package migratetest

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"

	"github.com/ladzaretti/migrate"
)

// Example demonstrates acceptance testing of the provided [migrate.SQLiteDialect] dialect.
func ExampleTestDialect() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		fmt.Printf("open: %v", err)
		return
	}
	defer func() { //nolint:wsl // false positive
		_ = db.Close()
	}()

	if err := TestDialect(context.Background(), db, migrate.SQLiteDialect{}); err != nil {
		fmt.Printf("TestDialect: %v", err)
	}

	// Output:
}

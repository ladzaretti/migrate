package migrate

import (
	"github.com/ladzaretti/migrate/types"
)

type SQLiteDialect struct{}

var _ types.Dialect = SQLiteDialect{}

func (d SQLiteDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE
			IF NOT EXISTS schema_version (
				id INTEGER PRIMARY KEY CHECK (id = 0),
				version INTEGER,
				checksum TEXT NOT NULL
			);
		`
}

func (d SQLiteDialect) CurrentVersionQuery() string {
	return `SELECT id, version, checksum FROM schema_version;`
}

func (d SQLiteDialect) SaveVersionQuery() string {
	return `
        	INSERT INTO schema_version (id, version, checksum)
        	VALUES (0, $1, $2)
        	ON CONFLICT(id) 
        	DO UPDATE SET version = EXCLUDED.version, checksum = EXCLUDED.checksum;
	`
}

type PostgreSQLDialect struct{}

var _ types.Dialect = PostgreSQLDialect{}

func (d PostgreSQLDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE
			IF NOT EXISTS schema_version (
				id INTEGER PRIMARY KEY CHECK (id = 0),
				version INTEGER,
				checksum TEXT NOT NULL
			);
	`
}

func (d PostgreSQLDialect) CurrentVersionQuery() string {
	return `SELECT id, version, checksum FROM schema_version;`
}

func (d PostgreSQLDialect) SaveVersionQuery() string {
	return `
		INSERT INTO schema_version (id, version, checksum)
		VALUES (0, $1, $2)
		ON CONFLICT (id) 
		DO UPDATE SET version = EXCLUDED.version, checksum = EXCLUDED.checksum;
	`
}

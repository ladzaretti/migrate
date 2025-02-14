package migrate

type DialectAdapter interface {
	CreateVersionTableQuery() string
	CurrentVersionQuery() string
	SaveVersionQuery() string
}

type SQLiteDialect struct{}

var _ DialectAdapter = SQLiteDialect{}

func (a SQLiteDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 0),
			version INTEGER,
			hash TEXT NOT NULL
		);
		INSERT INTO schema_version (id, version, hash) VALUES (0, 0, "") ON CONFLICT DO NOTHING;
	`
}

func (a SQLiteDialect) CurrentVersionQuery() string {
	return `SELECT version, hash FROM schema_version;`
}

func (a SQLiteDialect) SaveVersionQuery() string {
	return `
		UPDATE schema_version
		SET version = $1, hash = $2
		WHERE id = 0;
	`
}

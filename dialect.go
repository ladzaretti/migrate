package migrate

type DialectAdapter interface {
	CreateVersionTableQuery() string
	CurrentVersionQuery() string
	SaveVersionQuery() string
}

type SQLiteDialect struct{}

var _ DialectAdapter = SQLiteDialect{}

func (d SQLiteDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 0),
			version INTEGER,
			checksum TEXT NOT NULL
		);
		INSERT INTO schema_version (id, version, checksum) VALUES (0, 0, "") ON CONFLICT DO NOTHING;
	`
}

func (d SQLiteDialect) CurrentVersionQuery() string {
	return `SELECT version, checksum FROM schema_version;`
}

func (d SQLiteDialect) SaveVersionQuery() string {
	return `
		UPDATE schema_version
		SET version = $1, checksum = $2
		WHERE id = 0;
	`
}

type PostgreSQLDialect struct{}

var _ DialectAdapter = PostgreSQLDialect{}

func (d PostgreSQLDialect) CreateVersionTableQuery() string {
	return `
		CREATE TABLE IF NOT EXISTS schema_version (
			id INTEGER PRIMARY KEY CHECK (id = 0),
			version INTEGER,
			checksum TEXT NOT NULL
		);
		INSERT INTO schema_version (id, version, checksum) 
		VALUES (0, 0, '') 
		ON CONFLICT (id) DO NOTHING;
	`
}

func (d PostgreSQLDialect) CurrentVersionQuery() string {
	return `SELECT version, checksum FROM schema_version;`
}

func (d PostgreSQLDialect) SaveVersionQuery() string {
	return `
		UPDATE schema_version
		SET version = $1, checksum = $2
		WHERE id = 0;
	`
}

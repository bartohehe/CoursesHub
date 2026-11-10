package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB wraps the SQLite connection pool used by the app.
type DB struct {
	*sql.DB
}

// Open opens (or creates) the SQLite database at path and applies migrations.
// It is safe to call on every startup.
func Open(path string) (*DB, error) {
	// _pragma options enable WAL + foreign keys via the modernc driver.
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)&_pragma=busy_timeout(5000)", path)

	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles a single writer; keep the pool small and predictable.
	sqlDB.SetMaxOpenConns(1)

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	d := &DB{sqlDB}
	if err := d.migrate(); err != nil {
		return nil, err
	}
	return d, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS courses (
  id          INTEGER PRIMARY KEY,
  slug        TEXT UNIQUE NOT NULL,
  title       TEXT NOT NULL,
  description TEXT,
  path        TEXT NOT NULL,
  category    TEXT
);

CREATE TABLE IF NOT EXISTS progress (
  course_id       INTEGER PRIMARY KEY REFERENCES courses(id),
  status          TEXT NOT NULL DEFAULT 'not_started',
  last_visited_at TIMESTAMP,
  completed_at    TIMESTAMP
);
`

func (d *DB) migrate() error {
	if _, err := d.Exec(schema); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}

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
	if err := d.removeLegacySeed(); err != nil {
		return nil, err
	}
	return d, nil
}

// removeLegacySeed deletes the four example courses that early versions seeded
// into every new database. They are matched by slug AND by the old path format
// ("/courses/<slug>/index.html"); uploaded courses use "/courses/<slug>/", so
// a real course with the same slug is never touched.
func (d *DB) removeLegacySeed() error {
	const match = `slug IN ('web-scraping', 'ml-fundamentals', 'data-stack', 'decomposition')
		AND path = '/courses/' || slug || '/index.html'`

	tx, err := d.Begin()
	if err != nil {
		return fmt.Errorf("remove legacy seed: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM progress WHERE course_id IN (SELECT id FROM courses WHERE ` + match + `)`); err != nil {
		return fmt.Errorf("remove legacy seed progress: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM courses WHERE ` + match); err != nil {
		return fmt.Errorf("remove legacy seed courses: %w", err)
	}
	return tx.Commit()
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

package db

import (
	"database/sql"
	"strings"

	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode = WAL;

CREATE TABLE IF NOT EXISTS classes (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	year    INTEGER NOT NULL CHECK(year BETWEEN 1 AND 4),
	label   TEXT NOT NULL,
	teacher TEXT NOT NULL,
	UNIQUE(year, label)
);

CREATE TABLE IF NOT EXISTS students (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT NOT NULL,
	class_id INTEGER NOT NULL REFERENCES classes(id)
);

CREATE TABLE IF NOT EXISTS books (
	id             INTEGER PRIMARY KEY AUTOINCREMENT,
	isbn           TEXT,
	title          TEXT NOT NULL,
	author         TEXT,
	publisher      TEXT,
	published_year INTEGER,
	cover_url      TEXT,
	category       TEXT,
	reading_level  TEXT
);

CREATE TABLE IF NOT EXISTS copies (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id    INTEGER NOT NULL REFERENCES books(id),
	nfc_tag_id TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS loans (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	student_id  INTEGER NOT NULL REFERENCES students(id),
	copy_id     INTEGER NOT NULL REFERENCES copies(id),
	borrowed_at TEXT NOT NULL DEFAULT (datetime('now')),
	due_at      TEXT NOT NULL,
	returned_at TEXT
);

CREATE TABLE IF NOT EXISTS settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS reservations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id      INTEGER NOT NULL REFERENCES books(id),
	student_id   INTEGER NOT NULL REFERENCES students(id),
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	cancelled_at TEXT
);

CREATE TABLE IF NOT EXISTS book_positions (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id INTEGER NOT NULL UNIQUE REFERENCES books(id) ON DELETE CASCADE,
	row     TEXT,
	column  TEXT
);
`

// addCascadeToBookPositions rebuilds book_positions with ON DELETE CASCADE on
// book_id for databases created before that constraint existed — CREATE TABLE
// IF NOT EXISTS in schema only takes effect on fresh databases, and SQLite
// can't ALTER an existing foreign key in place, so the table has to be
// recreated (preserving any rows) instead.
func addCascadeToBookPositions(db *sql.DB) error {
	rows, err := db.Query(`PRAGMA foreign_key_list(book_positions)`)
	if err != nil {
		return err
	}
	var onDelete string
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			rows.Close()
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()

	if strings.EqualFold(onDelete, "CASCADE") {
		return nil // fresh database already got it from schema, or no FK row found
	}

	_, err = db.Exec(`
		ALTER TABLE book_positions RENAME TO book_positions_old;
		CREATE TABLE book_positions (
			id      INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL UNIQUE REFERENCES books(id) ON DELETE CASCADE,
			row     TEXT,
			column  TEXT
		);
		INSERT INTO book_positions (id, book_id, row, column)
			SELECT id, book_id, row, column FROM book_positions_old;
		DROP TABLE book_positions_old;
	`)
	return err
}

func Open(path string) (*sql.DB, error) {
	// _pragma=foreign_keys(1) applies to every pooled connection the driver
	// opens, not just the one that happens to run the first Exec — a plain
	// "PRAGMA foreign_keys = ON" statement only affects whichever single
	// connection executes it, so FK enforcement (and ON DELETE CASCADE)
	// would be silently inconsistent across concurrent requests otherwise.
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}
	if err := addCascadeToBookPositions(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

CREATE TABLE classes (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	year    INTEGER NOT NULL CHECK(year BETWEEN 1 AND 4),
	label   TEXT NOT NULL,
	teacher TEXT NOT NULL,
	UNIQUE(year, label)
);

CREATE TABLE students (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	name     TEXT NOT NULL,
	class_id INTEGER NOT NULL REFERENCES classes(id)
);

CREATE TABLE books (
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

CREATE TABLE copies (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id    INTEGER NOT NULL REFERENCES books(id),
	nfc_tag_id TEXT NOT NULL UNIQUE
);

CREATE TABLE loans (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	student_id  INTEGER NOT NULL REFERENCES students(id),
	copy_id     INTEGER NOT NULL REFERENCES copies(id),
	borrowed_at TEXT NOT NULL DEFAULT (datetime('now')),
	due_at      TEXT NOT NULL,
	returned_at TEXT
);

CREATE TABLE settings (
	key   TEXT PRIMARY KEY,
	value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE reservations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id      INTEGER NOT NULL REFERENCES books(id),
	student_id   INTEGER NOT NULL REFERENCES students(id),
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	cancelled_at TEXT
);

CREATE TABLE book_positions (
	id      INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id INTEGER NOT NULL UNIQUE REFERENCES books(id) ON DELETE CASCADE,
	row     TEXT,
	column  TEXT
);

CREATE TABLE IF NOT EXISTS reservations (
	id           INTEGER PRIMARY KEY AUTOINCREMENT,
	book_id      INTEGER NOT NULL REFERENCES books(id),
	student_id   INTEGER NOT NULL REFERENCES students(id),
	created_at   TEXT NOT NULL DEFAULT (datetime('now')),
	cancelled_at TEXT
);

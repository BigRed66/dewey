CREATE TABLE IF NOT EXISTS loans (
	id          INTEGER PRIMARY KEY AUTOINCREMENT,
	student_id  INTEGER NOT NULL REFERENCES students(id),
	copy_id     INTEGER NOT NULL REFERENCES copies(id),
	borrowed_at TEXT NOT NULL DEFAULT (datetime('now')),
	due_at      TEXT NOT NULL,
	returned_at TEXT
);

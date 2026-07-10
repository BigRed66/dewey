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

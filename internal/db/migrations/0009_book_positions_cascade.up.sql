-- SQLite can't ALTER a foreign key in place, so rebuild book_positions with
-- ON DELETE CASCADE on book_id, preserving any existing rows. Safe to run
-- whether or not the table already had CASCADE (fresh installs get it from
-- migration 0008 already; this just rebuilds it again onto the same shape).
DROP TABLE IF EXISTS book_positions_old;

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

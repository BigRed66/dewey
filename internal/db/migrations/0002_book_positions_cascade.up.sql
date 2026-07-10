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

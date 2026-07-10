package store

import (
	"context"
	"database/sql"
	"errors"
)

type Book struct {
	ID            int64
	ISBN          string
	Title         string
	Author        string
	Publisher     string
	PublishedYear int
	CoverURL      string
	Category      string
	ReadingLevel  string
	CopyCount     int
	OnLoanCount   int
	ShelfRow      string
	ShelfColumn   string
}

func (s *Store) ListBooks(ctx context.Context) ([]Book, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,''),
		       COALESCE(b.publisher,''), COALESCE(b.published_year,0),
		       COALESCE(b.cover_url,''), COALESCE(b.category,''), COALESCE(b.reading_level,''),
		       (SELECT COUNT(*) FROM copies c WHERE c.book_id = b.id),
		       (SELECT COUNT(*) FROM copies c JOIN loans l ON l.copy_id = c.id
		        WHERE c.book_id = b.id AND l.returned_at IS NULL),
		       COALESCE(p.row,''), COALESCE(p.column,'')
		FROM books b
		LEFT JOIN book_positions p ON p.book_id = b.id
		ORDER BY b.title`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var books []Book
	for rows.Next() {
		var b Book
		if err := rows.Scan(
			&b.ID, &b.ISBN, &b.Title, &b.Author,
			&b.Publisher, &b.PublishedYear, &b.CoverURL,
			&b.Category, &b.ReadingLevel,
			&b.CopyCount, &b.OnLoanCount,
			&b.ShelfRow, &b.ShelfColumn,
		); err != nil {
			return nil, err
		}
		books = append(books, b)
	}
	return books, rows.Err()
}

func (s *Store) GetBook(ctx context.Context, id int64) (Book, error) {
	var b Book
	err := s.db.QueryRowContext(ctx, `
		SELECT b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,''),
		       COALESCE(b.publisher,''), COALESCE(b.published_year,0), COALESCE(b.cover_url,''),
		       COALESCE(b.category,''), COALESCE(b.reading_level,''),
		       COALESCE(p.row,''), COALESCE(p.column,'')
		FROM books b
		LEFT JOIN book_positions p ON p.book_id = b.id
		WHERE b.id = ?`, id).
		Scan(&b.ID, &b.ISBN, &b.Title, &b.Author,
			&b.Publisher, &b.PublishedYear, &b.CoverURL, &b.Category, &b.ReadingLevel,
			&b.ShelfRow, &b.ShelfColumn)
	return b, err
}

func (s *Store) GetBookByISBN(ctx context.Context, isbn string) (Book, error) {
	var b Book
	err := s.db.QueryRowContext(ctx, `
		SELECT b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,''),
		       COALESCE(b.publisher,''), COALESCE(b.published_year,0), COALESCE(b.cover_url,''),
		       COALESCE(b.category,''), COALESCE(b.reading_level,''),
		       COALESCE(p.row,''), COALESCE(p.column,'')
		FROM books b
		LEFT JOIN book_positions p ON p.book_id = b.id
		WHERE b.isbn = ? LIMIT 1`, isbn).
		Scan(&b.ID, &b.ISBN, &b.Title, &b.Author,
			&b.Publisher, &b.PublishedYear, &b.CoverURL, &b.Category, &b.ReadingLevel,
			&b.ShelfRow, &b.ShelfColumn)
	return b, err
}

func (s *Store) CreateBook(ctx context.Context, b Book) (Book, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO books (isbn, title, author, publisher, published_year, cover_url, category, reading_level)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		nullStr(b.ISBN), b.Title, nullStr(b.Author), nullStr(b.Publisher),
		nullInt(b.PublishedYear), nullStr(b.CoverURL), nullStr(b.Category), nullStr(b.ReadingLevel))
	if err != nil {
		return Book{}, err
	}
	b.ID, _ = res.LastInsertId()
	return b, nil
}

func (s *Store) UpdateBook(ctx context.Context, b Book) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE books SET isbn=?, title=?, author=?, publisher=?, published_year=?,
		                 cover_url=?, category=?, reading_level=?
		WHERE id=?`,
		nullStr(b.ISBN), b.Title, nullStr(b.Author), nullStr(b.Publisher),
		nullInt(b.PublishedYear), nullStr(b.CoverURL), nullStr(b.Category), nullStr(b.ReadingLevel),
		b.ID)
	return err
}

var ErrBookOnLoan = errors.New("book has copies currently on loan")

func (s *Store) DeleteBook(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var onLoan int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM copies c JOIN loans l ON l.copy_id = c.id
		WHERE c.book_id = ? AND l.returned_at IS NULL`, id).Scan(&onLoan); err != nil {
		return err
	}
	if onLoan > 0 {
		return ErrBookOnLoan
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM loans WHERE copy_id IN (SELECT id FROM copies WHERE book_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM copies WHERE book_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM reservations WHERE book_id = ?`, id); err != nil {
		return err
	}
	// book_positions has ON DELETE CASCADE, so it's cleaned up automatically.
	if _, err := tx.ExecContext(ctx, `DELETE FROM books WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullInt(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

var ErrNotFound = sql.ErrNoRows

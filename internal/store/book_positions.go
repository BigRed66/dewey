package store

import "context"

// SetBookPosition records where a book is shelved (row + column). Passing
// empty strings for both clears the position again.
func (s *Store) SetBookPosition(ctx context.Context, bookID int64, row, column string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO book_positions (book_id, row, column) VALUES (?, ?, ?)
		ON CONFLICT(book_id) DO UPDATE SET row = excluded.row, column = excluded.column`,
		bookID, nullStr(row), nullStr(column))
	return err
}

package store

import (
	"context"
	"errors"
	"time"
)

type ScanResult struct {
	Found       bool
	Book        Book
	Copy        Copy
	OnLoan      bool
	StudentName string
	DueAt       time.Time
	DaysLeft    int
	DaysOverdue int
	Overdue     bool
}

func (s *Store) ScanByNFC(ctx context.Context, nfcTagID string) (ScanResult, error) {
	copy, err := s.GetCopyByNFCTag(ctx, nfcTagID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ScanResult{Found: false}, nil
		}
		return ScanResult{}, err
	}

	book, err := s.GetBook(ctx, copy.BookID)
	if err != nil {
		return ScanResult{}, err
	}

	result := ScanResult{Found: true, Book: book, Copy: copy}

	var studentName, dueStr string
	err = s.db.QueryRowContext(ctx, `
		SELECT s.name, l.due_at
		FROM loans l
		JOIN students s ON s.id = l.student_id
		WHERE l.copy_id = ? AND l.returned_at IS NULL
		LIMIT 1`, copy.ID).Scan(&studentName, &dueStr)

	if err == nil {
		result.OnLoan = true
		result.StudentName = studentName
		result.DueAt, _ = time.Parse(time.RFC3339, dueStr)
		diff := time.Until(result.DueAt).Hours() / 24
		if diff < 0 {
			result.Overdue = true
			result.DaysOverdue = int(-diff)
		} else {
			result.DaysLeft = int(diff)
		}
	} else if !errors.Is(err, ErrNotFound) {
		return ScanResult{}, err
	}

	return result, nil
}

type Copy struct {
	ID        int64
	BookID    int64
	NFCTagID  string
	Book      *Book
}

func (s *Store) CreateCopy(ctx context.Context, bookID int64, nfcTagID string) (Copy, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO copies (book_id, nfc_tag_id) VALUES (?, ?)`, bookID, nfcTagID)
	if err != nil {
		return Copy{}, err
	}
	id, _ := res.LastInsertId()
	return Copy{ID: id, BookID: bookID, NFCTagID: nfcTagID}, nil
}

func (s *Store) GetCopyByNFCTag(ctx context.Context, nfcTagID string) (Copy, error) {
	var c Copy
	err := s.db.QueryRowContext(ctx,
		`SELECT id, book_id, nfc_tag_id FROM copies WHERE nfc_tag_id = ?`, nfcTagID).
		Scan(&c.ID, &c.BookID, &c.NFCTagID)
	return c, err
}

func (s *Store) ListCopiesByBook(ctx context.Context, bookID int64) ([]Copy, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, book_id, nfc_tag_id FROM copies WHERE book_id = ? ORDER BY id`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var copies []Copy
	for rows.Next() {
		var c Copy
		if err := rows.Scan(&c.ID, &c.BookID, &c.NFCTagID); err != nil {
			return nil, err
		}
		copies = append(copies, c)
	}
	return copies, rows.Err()
}

func (s *Store) DeleteCopy(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM loans WHERE copy_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM copies WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) NFCTagExists(ctx context.Context, nfcTagID string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM copies WHERE nfc_tag_id = ?`, nfcTagID).Scan(&n)
	return n > 0, err
}

func (s *Store) CopyOnLoan(ctx context.Context, copyID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM loans WHERE copy_id = ? AND returned_at IS NULL`, copyID).Scan(&n)
	return n > 0, err
}

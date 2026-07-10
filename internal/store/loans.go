package store

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

const LoanDays = 14

// ErrLoanReserved is returned when trying to prolong a loan whose book has
// an active reservation from another student — they're waiting for it back.
var ErrLoanReserved = errors.New("another student has reserved this book")

// ErrInvalidDueDate is returned when prolonging a loan to a date that isn't
// after today.
var ErrInvalidDueDate = errors.New("due date must be in the future")

type Loan struct {
	ID         int64
	StudentID  int64
	CopyID     int64
	BorrowedAt time.Time
	DueAt      time.Time
	ReturnedAt *time.Time
	Student    *Student
	Book       *Book
	Copy       *Copy
}

func (s *Store) Checkout(ctx context.Context, studentID, copyID int64, dueAt time.Time) (Loan, error) {
	now := time.Now().UTC()

	res, err := s.db.ExecContext(ctx, `
		INSERT INTO loans (student_id, copy_id, borrowed_at, due_at)
		VALUES (?, ?, ?, ?)`,
		studentID, copyID,
		now.Format(time.RFC3339),
		dueAt.Format(time.RFC3339))
	if err != nil {
		return Loan{}, err
	}
	id, _ := res.LastInsertId()
	return Loan{
		ID:         id,
		StudentID:  studentID,
		CopyID:     copyID,
		BorrowedAt: now,
		DueAt:      dueAt,
	}, nil
}

// Return closes the open loan for the copy identified by nfcTagID.
func (s *Store) Return(ctx context.Context, nfcTagID string) (Loan, error) {
	var copyID int64
	if err := s.db.QueryRowContext(ctx,
		`SELECT id FROM copies WHERE nfc_tag_id = ?`, nfcTagID).Scan(&copyID); err != nil {
		return Loan{}, ErrNotFound
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE loans SET returned_at = ? WHERE copy_id = ? AND returned_at IS NULL`,
		now.Format(time.RFC3339), copyID)
	if err != nil {
		return Loan{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Loan{}, ErrNotFound
	}

	return Loan{CopyID: copyID, ReturnedAt: &now}, nil
}

func (s *Store) ActiveLoans(ctx context.Context) ([]Loan, error) {
	return s.queryLoans(ctx, `
		SELECT l.id, l.student_id, l.copy_id, l.borrowed_at, l.due_at,
		       s.name, s.class_id, c.year, c.label, c.teacher,
		       co.nfc_tag_id, b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,'')
		FROM loans l
		JOIN students s  ON s.id  = l.student_id
		JOIN classes  c  ON c.id  = s.class_id
		JOIN copies   co ON co.id = l.copy_id
		JOIN books    b  ON b.id  = co.book_id
		WHERE l.returned_at IS NULL
		ORDER BY l.due_at`)
}

func (s *Store) OverdueLoans(ctx context.Context) ([]Loan, error) {
	return s.queryLoans(ctx, `
		SELECT l.id, l.student_id, l.copy_id, l.borrowed_at, l.due_at,
		       s.name, s.class_id, c.year, c.label, c.teacher,
		       co.nfc_tag_id, b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,'')
		FROM loans l
		JOIN students s  ON s.id  = l.student_id
		JOIN classes  c  ON c.id  = s.class_id
		JOIN copies   co ON co.id = l.copy_id
		JOIN books    b  ON b.id  = co.book_id
		WHERE l.returned_at IS NULL AND l.due_at < datetime('now')
		ORDER BY l.due_at`)
}

func (s *Store) LoansByStudent(ctx context.Context, studentID int64) ([]Loan, error) {
	return s.queryLoans(ctx, `
		SELECT l.id, l.student_id, l.copy_id, l.borrowed_at, l.due_at,
		       s.name, s.class_id, c.year, c.label, c.teacher,
		       co.nfc_tag_id, b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,'')
		FROM loans l
		JOIN students s  ON s.id  = l.student_id
		JOIN classes  c  ON c.id  = s.class_id
		JOIN copies   co ON co.id = l.copy_id
		JOIN books    b  ON b.id  = co.book_id
		WHERE l.student_id = ?
		ORDER BY l.borrowed_at DESC`, studentID)
}

func (s *Store) AllLoansByBook(ctx context.Context, bookID int64) ([]Loan, error) {
	return s.queryAllLoans(ctx, `
		SELECT l.id, l.student_id, l.copy_id, l.borrowed_at, l.due_at, l.returned_at,
		       s.name, s.class_id, c.year, c.label, c.teacher,
		       co.nfc_tag_id, b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,'')
		FROM loans l
		JOIN students s  ON s.id  = l.student_id
		JOIN classes  c  ON c.id  = s.class_id
		JOIN copies   co ON co.id = l.copy_id
		JOIN books    b  ON b.id  = co.book_id
		WHERE b.id = ?
		ORDER BY l.borrowed_at DESC`, bookID)
}

func (s *Store) AllLoansByStudent(ctx context.Context, studentID int64) ([]Loan, error) {
	return s.queryAllLoans(ctx, `
		SELECT l.id, l.student_id, l.copy_id, l.borrowed_at, l.due_at, l.returned_at,
		       s.name, s.class_id, c.year, c.label, c.teacher,
		       co.nfc_tag_id, b.id, COALESCE(b.isbn,''), b.title, COALESCE(b.author,'')
		FROM loans l
		JOIN students s  ON s.id  = l.student_id
		JOIN classes  c  ON c.id  = s.class_id
		JOIN copies   co ON co.id = l.copy_id
		JOIN books    b  ON b.id  = co.book_id
		WHERE l.student_id = ?
		ORDER BY l.borrowed_at DESC`, studentID)
}

func (s *Store) queryAllLoans(ctx context.Context, query string, args ...interface{}) ([]Loan, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []Loan
	for rows.Next() {
		var l Loan
		var borrowedStr, dueStr string
		var returnedStr *string
		var st Student
		var cl Class
		var co Copy
		var b Book

		if err := rows.Scan(
			&l.ID, &l.StudentID, &l.CopyID, &borrowedStr, &dueStr, &returnedStr,
			&st.Name, &st.ClassID, &cl.Year, &cl.Label, &cl.Teacher,
			&co.NFCTagID, &b.ID, &b.ISBN, &b.Title, &b.Author,
		); err != nil {
			return nil, err
		}

		l.BorrowedAt, _ = time.Parse(time.RFC3339, borrowedStr)
		l.DueAt, _ = time.Parse(time.RFC3339, dueStr)
		if returnedStr != nil {
			t, _ := time.Parse(time.RFC3339, *returnedStr)
			l.ReturnedAt = &t
		}

		cl.ID = st.ClassID
		st.ID = l.StudentID
		st.Class = &cl
		co.ID = l.CopyID
		co.BookID = b.ID
		co.Book = &b
		l.Student = &st
		l.Copy = &co
		l.Book = &b

		loans = append(loans, l)
	}
	return loans, rows.Err()
}

// ProlongLoan sets an open loan's due date to newDue, unless another student
// has an active reservation on the book — in that case the current borrower
// must return it instead of renewing.
func (s *Store) ProlongLoan(ctx context.Context, loanID int64, newDue time.Time) (time.Time, error) {
	if !newDue.After(time.Now()) {
		return time.Time{}, ErrInvalidDueDate
	}

	var studentID, bookID int64
	var returnedAt sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT l.student_id, c.book_id, l.returned_at
		FROM loans l JOIN copies c ON c.id = l.copy_id
		WHERE l.id = ?`, loanID).Scan(&studentID, &bookID, &returnedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, ErrNotFound
		}
		return time.Time{}, err
	}
	if returnedAt.Valid {
		return time.Time{}, ErrNotFound
	}

	reserved, err := s.ReservedByOtherStudent(ctx, bookID, studentID)
	if err != nil {
		return time.Time{}, err
	}
	if reserved {
		return time.Time{}, ErrLoanReserved
	}

	if _, err := s.db.ExecContext(ctx,
		`UPDATE loans SET due_at = ? WHERE id = ?`, newDue.Format(time.RFC3339), loanID); err != nil {
		return time.Time{}, err
	}
	return newDue, nil
}

func (s *Store) queryLoans(ctx context.Context, query string, args ...interface{}) ([]Loan, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loans []Loan
	for rows.Next() {
		var l Loan
		var borrowedStr, dueStr string
		var st Student
		var cl Class
		var co Copy
		var b Book

		if err := rows.Scan(
			&l.ID, &l.StudentID, &l.CopyID, &borrowedStr, &dueStr,
			&st.Name, &st.ClassID, &cl.Year, &cl.Label, &cl.Teacher,
			&co.NFCTagID, &b.ID, &b.ISBN, &b.Title, &b.Author,
		); err != nil {
			return nil, err
		}

		l.BorrowedAt, _ = time.Parse(time.RFC3339, borrowedStr)
		l.DueAt, _ = time.Parse(time.RFC3339, dueStr)

		cl.ID = st.ClassID
		st.ID = l.StudentID
		st.Class = &cl

		co.ID = l.CopyID
		co.BookID = b.ID
		co.Book = &b

		l.Student = &st
		l.Copy = &co
		l.Book = &b

		loans = append(loans, l)
	}
	return loans, rows.Err()
}

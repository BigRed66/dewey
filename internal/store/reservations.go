package store

import (
	"context"
	"errors"
	"time"
)

// ErrReservationExists is returned when the same student already has an
// active (uncancelled) reservation on the same book.
var ErrReservationExists = errors.New("student already has an active reservation for this book")

type Reservation struct {
	ID        int64
	BookID    int64
	StudentID int64
	CreatedAt time.Time
	Book      *Book
	Student   *Student
}

// CreateReservation places a "Vormerkung" — a student registering interest
// in a book so that whoever currently holds it knows someone is waiting.
func (s *Store) CreateReservation(ctx context.Context, bookID, studentID int64) (Reservation, error) {
	var n int
	if err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM reservations
		WHERE book_id = ? AND student_id = ? AND cancelled_at IS NULL`, bookID, studentID).Scan(&n); err != nil {
		return Reservation{}, err
	}
	if n > 0 {
		return Reservation{}, ErrReservationExists
	}

	now := time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO reservations (book_id, student_id, created_at) VALUES (?, ?, ?)`,
		bookID, studentID, now.Format(time.RFC3339))
	if err != nil {
		return Reservation{}, err
	}
	id, _ := res.LastInsertId()
	return Reservation{ID: id, BookID: bookID, StudentID: studentID, CreatedAt: now}, nil
}

func (s *Store) CancelReservation(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE reservations SET cancelled_at = datetime('now') WHERE id = ? AND cancelled_at IS NULL`, id)
	return err
}

func (s *Store) ActiveReservationsByBook(ctx context.Context, bookID int64) ([]Reservation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.book_id, r.student_id, r.created_at,
		       s.name, s.class_id, c.year, c.label, c.teacher
		FROM reservations r
		JOIN students s ON s.id = r.student_id
		JOIN classes  c ON c.id = s.class_id
		WHERE r.book_id = ? AND r.cancelled_at IS NULL
		ORDER BY r.created_at`, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReservations(rows)
}

func (s *Store) ActiveReservationsByStudent(ctx context.Context, studentID int64) ([]Reservation, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT r.id, r.book_id, r.student_id, r.created_at,
		       b.title, b.author
		FROM reservations r
		JOIN books b ON b.id = r.book_id
		WHERE r.student_id = ? AND r.cancelled_at IS NULL
		ORDER BY r.created_at`, studentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Reservation
	for rows.Next() {
		var r Reservation
		var createdStr string
		var b Book
		if err := rows.Scan(&r.ID, &r.BookID, &r.StudentID, &createdStr, &b.Title, &b.Author); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		b.ID = r.BookID
		r.Book = &b
		out = append(out, r)
	}
	return out, rows.Err()
}

// ReservedByOtherStudent reports whether another student (not excludeStudentID)
// has an active reservation on bookID — used to block prolonging a loan.
func (s *Store) ReservedByOtherStudent(ctx context.Context, bookID, excludeStudentID int64) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM reservations
		WHERE book_id = ? AND student_id != ? AND cancelled_at IS NULL`,
		bookID, excludeStudentID).Scan(&n)
	return n > 0, err
}

func scanReservations(rows interface {
	Next() bool
	Scan(...interface{}) error
	Err() error
}) ([]Reservation, error) {
	var out []Reservation
	for rows.Next() {
		var r Reservation
		var createdStr string
		var st Student
		var cl Class
		if err := rows.Scan(&r.ID, &r.BookID, &r.StudentID, &createdStr,
			&st.Name, &st.ClassID, &cl.Year, &cl.Label, &cl.Teacher); err != nil {
			return nil, err
		}
		r.CreatedAt, _ = time.Parse(time.RFC3339, createdStr)
		cl.ID = st.ClassID
		st.ID = r.StudentID
		st.Class = &cl
		r.Student = &st
		out = append(out, r)
	}
	return out, rows.Err()
}

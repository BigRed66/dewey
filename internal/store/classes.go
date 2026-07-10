package store

import (
	"context"
	"errors"
)

type Class struct {
	ID      int64
	Year    int
	Label   string
	Teacher string
}

func (s *Store) ListClasses(ctx context.Context) ([]Class, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, year, label, teacher FROM classes ORDER BY year, label`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var classes []Class
	for rows.Next() {
		var c Class
		if err := rows.Scan(&c.ID, &c.Year, &c.Label, &c.Teacher); err != nil {
			return nil, err
		}
		classes = append(classes, c)
	}
	return classes, rows.Err()
}

func (s *Store) CreateClass(ctx context.Context, year int, label, teacher string) (Class, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO classes (year, label, teacher) VALUES (?, ?, ?)`,
		year, label, teacher)
	if err != nil {
		return Class{}, err
	}
	id, _ := res.LastInsertId()
	return Class{ID: id, Year: year, Label: label, Teacher: teacher}, nil
}

func (s *Store) GetClass(ctx context.Context, id int64) (Class, error) {
	var c Class
	err := s.db.QueryRowContext(ctx,
		`SELECT id, year, label, teacher FROM classes WHERE id = ?`, id).
		Scan(&c.ID, &c.Year, &c.Label, &c.Teacher)
	return c, err
}

func (s *Store) UpdateClass(ctx context.Context, id int64, year int, label, teacher string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE classes SET year = ?, label = ?, teacher = ? WHERE id = ?`,
		year, label, teacher, id)
	return err
}

var ErrClassOnLoan = errors.New("class has students with an active loan")

func (s *Store) DeleteClass(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var onLoan int
	if err := tx.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM loans l
		JOIN students s ON s.id = l.student_id
		WHERE s.class_id = ? AND l.returned_at IS NULL`, id).Scan(&onLoan); err != nil {
		return err
	}
	if onLoan > 0 {
		return ErrClassOnLoan
	}

	if _, err := tx.ExecContext(ctx, `
		DELETE FROM loans WHERE student_id IN (SELECT id FROM students WHERE class_id = ?)`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM students WHERE class_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM classes WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

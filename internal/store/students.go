package store

import (
	"context"
	"errors"
)

type Student struct {
	ID      int64
	Name    string
	ClassID int64
	Class   *Class
}

func (s *Store) ListStudents(ctx context.Context) ([]Student, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.class_id, c.year, c.label, c.teacher
		FROM students s
		JOIN classes c ON c.id = s.class_id
		ORDER BY c.year, c.label, s.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []Student
	for rows.Next() {
		var st Student
		var c Class
		if err := rows.Scan(&st.ID, &st.Name, &st.ClassID, &c.Year, &c.Label, &c.Teacher); err != nil {
			return nil, err
		}
		c.ID = st.ClassID
		st.Class = &c
		students = append(students, st)
	}
	return students, rows.Err()
}

func (s *Store) ListStudentsByClass(ctx context.Context, classID int64) ([]Student, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, class_id FROM students WHERE class_id = ? ORDER BY name`, classID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var students []Student
	for rows.Next() {
		var st Student
		if err := rows.Scan(&st.ID, &st.Name, &st.ClassID); err != nil {
			return nil, err
		}
		students = append(students, st)
	}
	return students, rows.Err()
}

func (s *Store) CreateStudent(ctx context.Context, name string, classID int64) (Student, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO students (name, class_id) VALUES (?, ?)`, name, classID)
	if err != nil {
		return Student{}, err
	}
	id, _ := res.LastInsertId()
	return Student{ID: id, Name: name, ClassID: classID}, nil
}

func (s *Store) GetStudent(ctx context.Context, id int64) (Student, error) {
	var st Student
	var c Class
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.name, s.class_id, c.year, c.label, c.teacher
		FROM students s
		JOIN classes c ON c.id = s.class_id
		WHERE s.id = ?`, id).
		Scan(&st.ID, &st.Name, &st.ClassID, &c.Year, &c.Label, &c.Teacher)
	if err != nil {
		return Student{}, err
	}
	c.ID = st.ClassID
	st.Class = &c
	return st, nil
}

func (s *Store) UpdateStudent(ctx context.Context, id int64, name string, classID int64) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE students SET name = ?, class_id = ? WHERE id = ?`, name, classID, id)
	return err
}

var ErrStudentOnLoan = errors.New("student has an active loan")

func (s *Store) DeleteStudent(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var onLoan int
	if err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM loans WHERE student_id = ? AND returned_at IS NULL`, id).Scan(&onLoan); err != nil {
		return err
	}
	if onLoan > 0 {
		return ErrStudentOnLoan
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM loans WHERE student_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM students WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

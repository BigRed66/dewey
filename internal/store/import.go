package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ImportTeacherPlaceholder fills classes/teacher for classes created by a CSV
// import — pupil exports carry no teacher name, and the column is NOT NULL.
// It's deliberately conspicuous so it's obvious what still needs editing.
const ImportTeacherPlaceholder = "Noch einzutragen"

// ImportStudent is one pupil to import, with the class named the way the CSV
// spells it rather than by id — the class is created on demand.
type ImportStudent struct {
	Name       string
	ClassYear  int
	ClassLabel string
}

// ImportSummary reports what an import actually changed.
type ImportSummary struct {
	ClassesCreated  int
	StudentsCreated int
	StudentsSkipped int
}

// ImportStudents creates any classes that don't exist yet and inserts the
// pupils, all in one transaction — a failure part-way leaves the database
// untouched rather than half-imported.
//
// Pupils that already exist (same name in the same class) are skipped, so
// re-importing an updated export tops up the roster instead of duplicating
// it.
func (s *Store) ImportStudents(ctx context.Context, students []ImportStudent) (ImportSummary, error) {
	var sum ImportSummary

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return sum, err
	}
	defer tx.Rollback()

	classIDs := make(map[string]int64)
	for _, st := range students {
		key := fmt.Sprintf("%d|%s", st.ClassYear, st.ClassLabel)
		classID, seen := classIDs[key]
		if !seen {
			err := tx.QueryRowContext(ctx,
				`SELECT id FROM classes WHERE year = ? AND label = ?`,
				st.ClassYear, st.ClassLabel).Scan(&classID)
			switch {
			case errors.Is(err, sql.ErrNoRows):
				res, err := tx.ExecContext(ctx,
					`INSERT INTO classes (year, label, teacher) VALUES (?, ?, ?)`,
					st.ClassYear, st.ClassLabel, ImportTeacherPlaceholder)
				if err != nil {
					return ImportSummary{}, fmt.Errorf("create class %d%s: %w", st.ClassYear, st.ClassLabel, err)
				}
				classID, _ = res.LastInsertId()
				sum.ClassesCreated++
			case err != nil:
				return ImportSummary{}, err
			}
			classIDs[key] = classID
		}

		var existing int
		if err := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM students WHERE name = ? AND class_id = ?`,
			st.Name, classID).Scan(&existing); err != nil {
			return ImportSummary{}, err
		}
		if existing > 0 {
			sum.StudentsSkipped++
			continue
		}

		if _, err := tx.ExecContext(ctx,
			`INSERT INTO students (name, class_id) VALUES (?, ?)`, st.Name, classID); err != nil {
			return ImportSummary{}, fmt.Errorf("create student %q: %w", st.Name, err)
		}
		sum.StudentsCreated++
	}

	if err := tx.Commit(); err != nil {
		return ImportSummary{}, err
	}
	return sum, nil
}

// Package seed fills a fresh database with realistic mock data for
// manual exploration and end-to-end tests.
package seed

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type class struct {
	Year    int
	Label   string
	Teacher string
}

type student struct {
	Name  string
	Class int // index into classes
}

type book struct {
	ISBN, Title, Author, Publisher string
	Year                           int
	Category, Level                string
	Copies                         int
	Row, Column                    string
}

var classes = []class{
	{1, "A", "Frau Bauer"},
	{2, "B", "Herr Klein"},
	{3, "A", "Frau Vogel"},
}

var students = []student{
	{"Anna Meier", 0}, {"Ben Schulz", 0}, {"Clara Fischer", 0},
	{"David Wolf", 1}, {"Emma Hoffmann", 1},
	{"Felix Krause", 2}, {"Greta Braun", 2}, {"Hannah Zimmer", 2}, {"Ian Weber", 2},
}

var books = []book{
	{"978-3-1234-0001-1", "Die kleine Raupe Nimmersatt", "Eric Carle", "Gerstenberg", 1994, "Bilderbuch", "Klasse 1", 2, "A", "1"},
	{"978-3-1234-0002-2", "Der kleine Prinz", "Antoine de Saint-Exupéry", "Karl Rauch", 1950, "Abenteuer", "Klasse 3", 2, "", ""},
	{"978-3-1234-0003-3", "Harry Potter und der Stein der Weisen", "J.K. Rowling", "Carlsen", 1998, "Fantasy", "Klasse 4", 3, "B", "2"},
	{"978-3-1234-0004-4", "Tierlexikon für Kinder", "", "Rowohlt", 2015, "Sachbuch", "Klasse 2", 1, "", ""},
	{"978-3-1234-0005-5", "Gregs Tagebuch", "Jeff Kinney", "Baumhaus", 2008, "Humor", "Klasse 3–4", 2, "", ""},
	{"978-3-1234-0006-6", "Die drei ???", "", "Kosmos", 1968, "Abenteuer", "Klasse 4", 1, "", ""},
}

// Run inserts mock classes, students, books, copies and loans into database.
// It assumes the schema is empty (or at least free of id collisions) — callers
// running this against a shared database should use a fresh file.
func Run(ctx context.Context, database *sql.DB) error {
	classIDs := make([]int64, len(classes))
	for i, c := range classes {
		res, err := database.ExecContext(ctx,
			`INSERT INTO classes (year, label, teacher) VALUES (?, ?, ?)`, c.Year, c.Label, c.Teacher)
		if err != nil {
			return fmt.Errorf("seed class %q: %w", c.Label, err)
		}
		classIDs[i], _ = res.LastInsertId()
	}

	studentIDs := make([]int64, len(students))
	for i, st := range students {
		res, err := database.ExecContext(ctx,
			`INSERT INTO students (name, class_id) VALUES (?, ?)`, st.Name, classIDs[st.Class])
		if err != nil {
			return fmt.Errorf("seed student %q: %w", st.Name, err)
		}
		studentIDs[i], _ = res.LastInsertId()
	}

	bookIDs := make([]int64, len(books))
	copyIDs := make(map[int][]int64, len(books))
	for i, b := range books {
		res, err := database.ExecContext(ctx, `
			INSERT INTO books (isbn, title, author, publisher, published_year, category, reading_level)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			b.ISBN, b.Title, nullStr(b.Author), b.Publisher, b.Year, b.Category, b.Level)
		if err != nil {
			return fmt.Errorf("seed book %q: %w", b.Title, err)
		}
		bookID, _ := res.LastInsertId()
		bookIDs[i] = bookID

		if b.Row != "" || b.Column != "" {
			if _, err := database.ExecContext(ctx,
				`INSERT INTO book_positions (book_id, row, column) VALUES (?, ?, ?)`,
				bookID, nullStr(b.Row), nullStr(b.Column)); err != nil {
				return fmt.Errorf("seed position for %q: %w", b.Title, err)
			}
		}

		for c := 0; c < b.Copies; c++ {
			tag := fmt.Sprintf("E2E-%d-%d", i+1, c+1)
			cres, err := database.ExecContext(ctx,
				`INSERT INTO copies (book_id, nfc_tag_id) VALUES (?, ?)`, bookID, tag)
			if err != nil {
				return fmt.Errorf("seed copy for %q: %w", b.Title, err)
			}
			copyID, _ := cres.LastInsertId()
			copyIDs[i] = append(copyIDs[i], copyID)
		}
	}

	now := time.Now().UTC()
	insertLoan := func(studentID, copyID int64, borrowed, due time.Time, returned *time.Time) error {
		var ret interface{}
		if returned != nil {
			ret = returned.Format(time.RFC3339)
		}
		_, err := database.ExecContext(ctx,
			`INSERT INTO loans (student_id, copy_id, borrowed_at, due_at, returned_at) VALUES (?, ?, ?, ?, ?)`,
			studentID, copyID, borrowed.Format(time.RFC3339), due.Format(time.RFC3339), ret)
		return err
	}

	// Active loan, not yet due — blocks deleting Anna, class 1A and "Harry Potter".
	if err := insertLoan(studentIDs[0], copyIDs[2][0],
		now.AddDate(0, 0, -3), now.AddDate(0, 0, 11), nil); err != nil {
		return fmt.Errorf("seed active loan: %w", err)
	}
	// Active, overdue loan — blocks deleting David, class 2B and "Der kleine Prinz".
	if err := insertLoan(studentIDs[3], copyIDs[1][0],
		now.AddDate(0, 0, -20), now.AddDate(0, 0, -6), nil); err != nil {
		return fmt.Errorf("seed overdue loan: %w", err)
	}
	// Returned loan history — Felix and "Raupe Nimmersatt" should still delete cleanly.
	returnedAt := now.AddDate(0, 0, -10)
	if err := insertLoan(studentIDs[5], copyIDs[0][0],
		now.AddDate(0, 0, -24), now.AddDate(0, 0, -10), &returnedAt); err != nil {
		return fmt.Errorf("seed returned loan: %w", err)
	}
	// Returned loan history for a student in the same class as an active loan
	// (Anna) — Clara should still be individually deletable even though her
	// class can't be deleted while Anna's loan is active.
	returnedAt2 := now.AddDate(0, 0, -2)
	if err := insertLoan(studentIDs[2], copyIDs[4][0],
		now.AddDate(0, 0, -16), now.AddDate(0, 0, -2), &returnedAt2); err != nil {
		return fmt.Errorf("seed second returned loan: %w", err)
	}

	return nil
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

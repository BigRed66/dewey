package db

import (
	"database/sql"
	"testing"
)

func TestOpenFreshDatabase(t *testing.T) {
	path := t.TempDir() + "/fresh.db"
	d, err := Open(path)
	if err != nil {
		t.Fatalf("open fresh: %v", err)
	}
	defer d.Close()

	assertMigrationVersion(t, d, 2)

	hasCascade, err := bookPositionsHasCascade(d)
	if err != nil {
		t.Fatalf("cascade check: %v", err)
	}
	if !hasCascade {
		t.Fatalf("expected book_positions.book_id to have ON DELETE CASCADE on a fresh db")
	}
}

// TestOpenLegacyDatabase simulates upgrading a database that was created by
// the pre-migration code: tables exist (via CREATE TABLE IF NOT EXISTS) but
// there's no schema_migrations bookkeeping, and book_positions predates the
// CASCADE fix. Open must recognize this, preserve existing rows, and bring
// the db under migration tracking without re-running CREATE TABLE.
func TestOpenLegacyDatabase(t *testing.T) {
	path := t.TempDir() + "/legacy.db"
	seed, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatalf("open seed connection: %v", err)
	}
	if _, err := seed.Exec(`
		CREATE TABLE classes (
			id INTEGER PRIMARY KEY AUTOINCREMENT, year INTEGER, label TEXT, teacher TEXT
		);
		CREATE TABLE students (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, class_id INTEGER);
		CREATE TABLE books (id INTEGER PRIMARY KEY AUTOINCREMENT, isbn TEXT, title TEXT);
		CREATE TABLE copies (id INTEGER PRIMARY KEY AUTOINCREMENT, book_id INTEGER, nfc_tag_id TEXT UNIQUE);
		CREATE TABLE loans (id INTEGER PRIMARY KEY AUTOINCREMENT, student_id INTEGER, copy_id INTEGER, due_at TEXT);
		CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT);
		CREATE TABLE reservations (id INTEGER PRIMARY KEY AUTOINCREMENT, book_id INTEGER, student_id INTEGER);
		CREATE TABLE book_positions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			book_id INTEGER NOT NULL UNIQUE REFERENCES books(id),
			row TEXT, column TEXT
		);
		INSERT INTO books (id, title) VALUES (1, 'Preserved Book');
		INSERT INTO book_positions (book_id, row, column) VALUES (1, 'A', '1');
	`); err != nil {
		t.Fatalf("seed legacy schema: %v", err)
	}
	if err := seed.Close(); err != nil {
		t.Fatalf("close seed connection: %v", err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatalf("open legacy: %v", err)
	}
	defer d.Close()

	assertMigrationVersion(t, d, 2)

	var title string
	if err := d.QueryRow(`SELECT title FROM books WHERE id = 1`).Scan(&title); err != nil {
		t.Fatalf("preserved book missing: %v", err)
	}
	if title != "Preserved Book" {
		t.Fatalf("expected preserved book title, got %q", title)
	}

	var row, col string
	if err := d.QueryRow(`SELECT row, column FROM book_positions WHERE book_id = 1`).Scan(&row, &col); err != nil {
		t.Fatalf("preserved book_position missing: %v", err)
	}
	if row != "A" || col != "1" {
		t.Fatalf("expected preserved position A/1, got %s/%s", row, col)
	}

	hasCascade, err := bookPositionsHasCascade(d)
	if err != nil {
		t.Fatalf("cascade check: %v", err)
	}
	if !hasCascade {
		t.Fatalf("expected book_positions.book_id to gain ON DELETE CASCADE after migrating a legacy db")
	}

	// Reopening must be idempotent: no re-application, no error.
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen legacy: %v", err)
	}
	defer d2.Close()
	assertMigrationVersion(t, d2, 2)
}

func assertMigrationVersion(t *testing.T, d *sql.DB, want int) {
	t.Helper()
	var version int
	var dirty bool
	if err := d.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&version, &dirty); err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	if dirty {
		t.Fatalf("schema_migrations reports dirty state")
	}
	if version != want {
		t.Fatalf("expected migration version %d, got %d", want, version)
	}
}

package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func Open(path string) (*sql.DB, error) {
	// _pragma=foreign_keys(1) applies to every pooled connection the driver
	// opens, not just the one that happens to run the first Exec — a plain
	// "PRAGMA foreign_keys = ON" statement only affects whichever single
	// connection executes it, so FK enforcement (and ON DELETE CASCADE)
	// would be silently inconsistent across concurrent requests otherwise.
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		db.Close()
		return nil, err
	}
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// runMigrations applies every migration under migrations/ that hasn't run
// yet. Each one is written to be safe to apply to a database that already
// has some or all of its tables — CREATE TABLE IF NOT EXISTS per table, plus
// an idempotent rebuild for the book_positions CASCADE fix — so upgrading a
// database that predates migration tracking just no-ops the steps it already
// satisfies instead of needing special-case detection.
func runMigrations(db *sql.DB) error {
	dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("init migration source: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("init migrator: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

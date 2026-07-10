package db

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database"
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

func runMigrations(db *sql.DB) error {
	dbDriver, err := migratesqlite.WithInstance(db, &migratesqlite.Config{})
	if err != nil {
		return fmt.Errorf("init migration driver: %w", err)
	}

	if err := bootstrapLegacyDatabase(db, dbDriver); err != nil {
		return fmt.Errorf("bootstrap legacy database: %w", err)
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

// bootstrapLegacyDatabase handles databases created by the pre-migration
// code (a const schema applied with CREATE TABLE IF NOT EXISTS, plus the
// one-off addCascadeToBookPositions rebuild). Those databases have tables
// but no migration version recorded, so migration 1 would otherwise try to
// CREATE TABLE over data that already exists. Detect that case once and mark
// the version it already satisfies as applied, without running any SQL.
func bootstrapLegacyDatabase(db *sql.DB, dbDriver database.Driver) error {
	version, _, err := dbDriver.Version()
	if err != nil {
		return err
	}
	if version != database.NilVersion {
		return nil // already under migration tracking
	}

	var tableExists int
	if err := db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'classes'`).Scan(&tableExists); err != nil {
		return err
	}
	if tableExists == 0 {
		return nil // fresh database, nothing to bootstrap
	}

	legacyVersion := 1
	hasCascade, err := bookPositionsHasCascade(db)
	if err != nil {
		return err
	}
	if hasCascade {
		legacyVersion = 2
	}
	return dbDriver.SetVersion(legacyVersion, false)
}

func bookPositionsHasCascade(db *sql.DB) (bool, error) {
	rows, err := db.Query(`PRAGMA foreign_key_list(book_positions)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var onDelete string
	for rows.Next() {
		var id, seq int
		var table, from, to, onUpdate, match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return false, err
		}
	}
	if err := rows.Err(); err != nil {
		return false, err
	}
	return strings.EqualFold(onDelete, "CASCADE"), nil
}

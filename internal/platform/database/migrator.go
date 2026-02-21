// Package database provides PostgreSQL connection pool management and
// migration support.
package database

import (
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	// Register the postgres driver for golang-migrate.
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	// Register the file source driver for reading migration files.
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

// Migrator wraps golang-migrate/migrate/v4 to run database migrations
// programmatically. It reads migration files from the configured source
// path and applies them against the target PostgreSQL database.
type Migrator struct {
	m *migrate.Migrate
}

// NewMigrator creates a new Migrator. sourcePath is the filesystem path to
// the migrations directory (e.g., "file://migrations"). databaseDSN is a
// full PostgreSQL connection string.
func NewMigrator(sourcePath, databaseDSN string) (*Migrator, error) {
	m, err := migrate.New(sourcePath, databaseDSN)
	if err != nil {
		return nil, fmt.Errorf("creating migrator: %w", err)
	}
	return &Migrator{m: m}, nil
}

// Up applies all available migrations that have not yet been run. It returns
// nil if the database is already up to date.
func (mg *Migrator) Up() error {
	err := mg.m.Up()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("running migrations up: %w", err)
	}
	return nil
}

// Down rolls back all applied migrations. Use with caution -- this reverses
// every migration back to a clean schema.
func (mg *Migrator) Down() error {
	err := mg.m.Down()
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("running migrations down: %w", err)
	}
	return nil
}

// Steps applies n migration steps. A positive n migrates up; a negative n
// migrates down. For example, Steps(1) applies the next pending migration,
// while Steps(-1) rolls back the most recently applied one.
func (mg *Migrator) Steps(n int) error {
	err := mg.m.Steps(n)
	if errors.Is(err, migrate.ErrNoChange) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stepping migrations by %d: %w", n, err)
	}
	return nil
}

// Version returns the currently applied migration version and whether the
// database is in a dirty state. A dirty state indicates that a previous
// migration failed partway through and manual intervention may be needed.
func (mg *Migrator) Version() (version uint, dirty bool, err error) {
	version, dirty, err = mg.m.Version()
	if errors.Is(err, migrate.ErrNoChange) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("getting migration version: %w", err)
	}
	return version, dirty, nil
}

// Close releases resources held by the underlying migrate instance. It
// should be called when the Migrator is no longer needed.
func (mg *Migrator) Close() error {
	srcErr, dbErr := mg.m.Close()
	if srcErr != nil {
		return fmt.Errorf("closing migration source: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("closing migration database: %w", dbErr)
	}
	return nil
}

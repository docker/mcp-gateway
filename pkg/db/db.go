package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/user"

	// This enables to sqlite driver
	_ "modernc.org/sqlite"
)

type DAO interface {
	WorkingSetDAO
	CatalogDAO
	MigrationStatusDAO
	PullRecordDAO

	// Normally unnecessary to call this
	Close() error
}

type dao struct {
	db *sqlx.DB
}

//go:embed migrations/*.sql
var migrations embed.FS

type options struct {
	dbFile         string
	migrationsFS   fs.FS
	migrationsPath string
}

type Option func(o *options) error

func WithDatabaseFile(dbFile string) Option {
	return func(o *options) error {
		o.dbFile = dbFile
		return nil
	}
}

func WithMigrations(filesystem fs.FS, path string) Option {
	return func(o *options) error {
		o.migrationsFS = filesystem
		o.migrationsPath = path
		return nil
	}
}

func New(opts ...Option) (DAO, error) {
	var o options
	for _, opt := range opts {
		if err := opt(&o); err != nil {
			return nil, err
		}
	}

	if o.dbFile == "" {
		dbFile, err := DefaultDatabaseFilename()
		if err != nil {
			return nil, fmt.Errorf("failed to get default database filename: %w", err)
		}
		o.dbFile = dbFile
	}

	ensureDirectoryExists(o.dbFile)

	db, err := sql.Open("sqlite", "file:"+o.dbFile+"?_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	migrationsFS := o.migrationsFS
	if migrationsFS == nil {
		migrationsFS = &migrations
	}

	migrationsPath := o.migrationsPath
	if migrationsPath == "" {
		migrationsPath = "migrations"
	}

	err = runMigrations(o.dbFile, db, migrationsFS, migrationsPath)
	if err != nil {
		return nil, err
	}

	sqlxDb := sqlx.NewDb(db, "sqlite")

	return &dao{db: sqlxDb}, nil
}

func (d *dao) Close() error {
	return d.db.Close()
}

func DefaultDatabaseFilename() (string, error) {
	homeDir, err := user.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".docker", "mcp", "mcp-toolkit.db"), nil
}

func ensureDirectoryExists(path string) {
	dir := filepath.Dir(path)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		_ = os.MkdirAll(dir, 0o755)
	}
}

func txClose(tx *sqlx.Tx, err *error) {
	if err == nil || *err == nil {
		return
	}

	if txerr := tx.Rollback(); txerr != nil {
		log.Logf("failed to rollback transaction: %v", txerr)
	}
}

func runMigrations(dbFile string, db *sql.DB, migrationsFS fs.FS, migrationsPath string) error {
	migDriver, err := iofs.New(migrationsFS, migrationsPath)
	if err != nil {
		return err
	}
	defer migDriver.Close()

	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		return err
	}

	mig, err := migrate.NewWithInstance("iofs", migDriver, "sqlite", driver)
	if err != nil {
		return err
	}

	// Use file locking to prevent concurrent migrations across processes
	// This ensures only one process can run migrations at a time, preventing
	// "Dirty database version" errors when multiple processes start simultaneously.
	// See docs/database/README.md
	//
	// Note: The lock file persists on disk after Unlock() - this is intentional.
	// flock.Unlock() only releases the lock and closes the file descriptor
	lockFile := filepath.Join(filepath.Dir(dbFile), ".mcp-toolkit-migration.lock")
	fileLock := flock.New(lockFile)

	// Try to acquire the lock with a 5 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	if !locked {
		return fmt.Errorf("timeout waiting for migration lock")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			log.Logf("failed to unlock migration lock: %v", err)
		}
	}()

	// Now that we have the lock, check the current migration state
	version, dirty, err := mig.Version()

	// If ErrNilVersion, the database is fresh (no migrations run yet)
	// In this case, proceed with running migrations
	isFreshDatabase := errors.Is(err, migrate.ErrNilVersion)

	if err != nil && !isFreshDatabase {
		return fmt.Errorf("failed to get migration version: %w", err)
	}

	// Check if database is in dirty state (migration was interrupted)
	if dirty {
		return fmt.Errorf("database is in dirty state at version %d, manual intervention required", version)
	}

	// For fresh databases, always run migrations
	if !isFreshDatabase {
		// Check if database version is ahead of available migrations
		// This happens when running older code against a database that was upgraded by newer code
		_, _, err = migDriver.ReadUp(version)
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("database version %d (%s) is ahead of the current application version. Please upgrade to the latest version", version, dbFile)
		}
		if err != nil {
			return fmt.Errorf("failed to read migration file for version %d: %w", version, err)
		}
	}

	// Now safely run migrations with the lock held
	err = mig.Up()
	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
	"github.com/golang-migrate/migrate/v4"
	msqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jmoiron/sqlx"

	"github.com/docker/mcp-gateway/pkg/log"
	"github.com/docker/mcp-gateway/pkg/retry"
	"github.com/docker/mcp-gateway/pkg/user"

	// This enables to sqlite driver
	_ "modernc.org/sqlite"
)

type DAO interface {
	WorkingSetDAO
	CatalogDAO
	MigrationStatusDAO

	// Normally unnecessary to call this
	Close() error
}

type dao struct {
	db *sqlx.DB
}

//go:embed migrations/*.sql
var migrations embed.FS

type options struct {
	dbFile string
}

type Option func(o *options) error

func WithDatabaseFile(dbFile string) Option {
	return func(o *options) error {
		o.dbFile = dbFile
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

	migDriver, err := iofs.New(migrations, "migrations")
	if err != nil {
		return nil, err
	}

	driver, err := msqlite.WithInstance(db, &msqlite.Config{})
	if err != nil {
		return nil, err
	}

	mig, err := migrate.NewWithInstance("iofs", migDriver, "sqlite", driver)
	if err != nil {
		return nil, err
	}

	// Use file locking to prevent concurrent migrations across processes
	// This ensures only one process can run migrations at a time, preventing
	// "Dirty database version" errors when multiple processes start simultaneously.
	// See docs/database/README.md
	//
	// Note: The lock file persists on disk after Unlock() - this is intentional.
	// flock.Unlock() only releases the lock and closes the file descriptor
	lockFile := filepath.Join(filepath.Dir(o.dbFile), ".mcp-toolkit-migration.lock")
	fileLock := flock.New(lockFile)

	// Try to acquire the lock with a 1 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	locked, err := fileLock.TryLockContext(ctx, 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire migration lock: %w", err)
	}
	if !locked {
		return nil, fmt.Errorf("timeout waiting for migration lock")
	}
	defer func() {
		if err := fileLock.Unlock(); err != nil {
			log.Logf("failed to unlock migration lock: %v", err)
		}
	}()

	// Now safely run migrations with the lock held
	// We still use a small retry in case of transient SQLite errors
	err = retry.Retry(3, 200*time.Millisecond, func() error {
		err := mig.Up()
		if err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
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

package db

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type MigrationStatusDAO interface {
	GetMigrationStatus(ctx context.Context) (*MigrationStatus, error)
	TryAcquireMigration(ctx context.Context, status string) (*MigrationStatus, bool, error)
	UpdateMigrationStatus(ctx context.Context, status MigrationStatus) error
}

type MigrationStatus struct {
	ID          *int64     `db:"id"`
	Status      string     `db:"status"`
	Logs        string     `db:"logs"`
	LastUpdated *time.Time `db:"last_updated"`
}

func (d *dao) GetMigrationStatus(ctx context.Context) (*MigrationStatus, error) {
	const query = `SELECT id, status, logs, last_updated FROM migration_status LIMIT 1`

	var migrationStatus MigrationStatus
	err := d.db.GetContext(ctx, &migrationStatus, query)
	if err != nil {
		return nil, err
	}
	return &migrationStatus, nil
}

// This will attempt to take ownership of the migration. If it returns true, the caller should do the migration.
func (d *dao) TryAcquireMigration(ctx context.Context, status string) (*MigrationStatus, bool, error) {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, false, err
	}
	defer txClose(tx, &err)

	const selectQuery = `SELECT id, status, logs, last_updated FROM migration_status LIMIT 1`

	var migrationStatus MigrationStatus
	err = tx.GetContext(ctx, &migrationStatus, selectQuery)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, false, err
		}
		// No rows, so insert instead
		const insertQuery = `INSERT INTO migration_status (status, logs) VALUES ($1, $2)`
		_, err = tx.ExecContext(ctx, insertQuery, status, "")
		if err != nil {
			return nil, false, err
		}

		err = tx.GetContext(ctx, &migrationStatus, selectQuery)
		if err != nil {
			return nil, false, err
		}

		if err = tx.Commit(); err != nil {
			return nil, false, err
		}

		// Acquired the migration status
		return &migrationStatus, true, nil
	}

	if err = tx.Commit(); err != nil {
		return nil, false, err
	}

	// Got an existing migration status
	return &migrationStatus, false, nil
}

func (d *dao) UpdateMigrationStatus(ctx context.Context, status MigrationStatus) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer txClose(tx, &err)

	const deleteQuery = `DELETE FROM migration_status`
	_, err = tx.ExecContext(ctx, deleteQuery)
	if err != nil {
		return err
	}

	const query = `INSERT INTO migration_status (status, logs) VALUES ($1, $2)`

	_, err = tx.ExecContext(ctx, query, status.Status, status.Logs)
	if err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

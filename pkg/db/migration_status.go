package db

import (
	"context"
	"time"
)

type MigrationStatusDAO interface {
	GetMigrationStatus(ctx context.Context) (*MigrationStatus, error)
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

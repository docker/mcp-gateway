package db

import (
	"context"
	"time"
)

type PullRecordDAO interface {
	RecordPull(ctx context.Context, ref string) error
	CheckPullRecord(ctx context.Context, ref string) (bool, error)
}

type PullRecord struct {
	ID          *int64     `db:"id"`
	Ref         string     `db:"ref"`
	FirstPulled *time.Time `db:"first_pulled"`
}

func (d *dao) RecordPull(ctx context.Context, ref string) error {
	// OR IGNORE avoids unique constraint errors. We don't care if the record already exists.
	const query = `INSERT OR IGNORE INTO pull_record (ref) VALUES ($1)`
	_, err := d.db.ExecContext(ctx, query, ref)
	return err
}

func (d *dao) CheckPullRecord(ctx context.Context, ref string) (bool, error) {
	const query = `SELECT COUNT(*) FROM pull_record WHERE ref = $1`
	var count int
	err := d.db.GetContext(ctx, &count, query, ref)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

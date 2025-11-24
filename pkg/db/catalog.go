package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type CatalogDAO interface {
	GetCatalog(ctx context.Context, ref string) (*Catalog, error)
	UpsertCatalog(ctx context.Context, catalog Catalog) error
	DeleteCatalog(ctx context.Context, ref string) error
	ListCatalogs(ctx context.Context) ([]Catalog, error)
}

type ToolList []string

type Catalog struct {
	Ref         string          `db:"ref"`
	Digest      string          `db:"digest"`
	Title       string          `db:"title"`
	Source      string          `db:"source"`
	LastUpdated *time.Time      `db:"last_updated"`
	Servers     []CatalogServer `db:"-"`
}

type CatalogServer struct {
	ID         *int64   `db:"id" json:"id"`
	ServerType string   `db:"server_type" json:"server_type"`
	Tools      ToolList `db:"tools" json:"tools"`
	Source     string   `db:"source" json:"source"`
	Image      string   `db:"image" json:"image"`
	Endpoint   string   `db:"endpoint" json:"endpoint"`
	CatalogRef string   `db:"catalog_ref" json:"catalog_ref"`

	Snapshot *ServerSnapshot `db:"snapshot" json:"snapshot"`
}

func (tools ToolList) Value() (driver.Value, error) {
	b, err := json.Marshal(tools)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (tools *ToolList) Scan(value any) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("failed to scan server list")
	}
	return json.Unmarshal([]byte(str), tools)
}

func (d *dao) GetCatalog(ctx context.Context, ref string) (*Catalog, error) {
	const query = `SELECT ref, digest, title, source, last_updated FROM catalog WHERE ref = $1`

	var catalog Catalog
	err := d.db.GetContext(ctx, &catalog, query, ref)
	if err != nil {
		return nil, err
	}

	const serverQuery = `SELECT id, server_type, tools, source, image, endpoint, catalog_ref, snapshot from catalog_server where catalog_ref = $1`

	var servers []CatalogServer
	err = d.db.SelectContext(ctx, &servers, serverQuery, catalog.Ref)
	if err != nil {
		return nil, err
	}
	catalog.Servers = servers

	return &catalog, nil
}

func (d *dao) UpsertCatalog(ctx context.Context, catalog Catalog) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer txClose(tx, &err)

	const deleteQuery = `DELETE FROM catalog WHERE ref = $1`

	_, err = tx.ExecContext(ctx, deleteQuery, catalog.Ref)
	if err != nil {
		return err
	}

	const insertQuery = `INSERT INTO catalog (ref, digest, title, source, last_updated) VALUES ($1, $2, $3, $4, current_timestamp)`

	_, err = tx.ExecContext(ctx, insertQuery, catalog.Ref, catalog.Digest, catalog.Title, catalog.Source)
	if err != nil {
		return err
	}

	for i := range catalog.Servers {
		catalog.Servers[i].CatalogRef = catalog.Ref
	}

	if len(catalog.Servers) > 0 {
		const serverQuery = `INSERT INTO catalog_server (
		server_type, tools, source, image, endpoint, catalog_ref, snapshot
	) VALUES (:server_type, :tools, :source, :image, :endpoint, :catalog_ref, :snapshot)`

		_, err = tx.NamedExecContext(ctx, serverQuery, catalog.Servers)
		if err != nil {
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (d *dao) DeleteCatalog(ctx context.Context, ref string) error {
	const query = `DELETE FROM catalog WHERE ref = $1`

	_, err := d.db.ExecContext(ctx, query, ref)
	if err != nil {
		return err
	}
	return nil
}

func (d *dao) ListCatalogs(ctx context.Context) ([]Catalog, error) {
	type catalogRow struct {
		Catalog
		ServerJSON string `db:"server_json"`
	}

	const query = `SELECT c.ref, c.digest, c.title, c.source, c.last_updated,
	COALESCE(
		json_group_array(json_object('id', s.id, 'server_type', s.server_type, 'tools', json(s.tools), 'source', s.source, 'image', s.image, 'endpoint', s.endpoint, 'snapshot', json(s.snapshot))),
		'[]'
	) AS server_json
	FROM catalog c
	LEFT JOIN catalog_server s ON s.catalog_ref = c.ref
	GROUP BY c.ref`

	var rows []catalogRow
	err := d.db.SelectContext(ctx, &rows, query)
	if err != nil {
		return nil, err
	}

	catalogs := make([]Catalog, len(rows))
	for i, row := range rows {
		if err := json.Unmarshal([]byte(row.ServerJSON), &row.Servers); err != nil {
			return nil, fmt.Errorf("failed to unmarshal servers: %w", err)
		}
		catalogs[i] = row.Catalog
	}

	return catalogs, nil
}

package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type CatalogDAO interface {
	GetCatalog(ctx context.Context, digest string) (*Catalog, error)
	CreateCatalog(ctx context.Context, catalog Catalog) error
	DeleteCatalog(ctx context.Context, digest string) error
	DeleteCatalogBySource(ctx context.Context, source string) error
	ListCatalogs(ctx context.Context) ([]Catalog, error)
}

type ToolList []string

type Catalog struct {
	ID      *int64          `db:"id"`
	Digest  string          `db:"digest"`
	Name    string          `db:"name"`
	Source  string          `db:"source"`
	Servers []CatalogServer `db:"-"`
}

type CatalogServer struct {
	ID         *int64   `db:"id" json:"id"`
	ServerType string   `db:"server_type" json:"server_type"`
	Tools      ToolList `db:"tools" json:"tools"`
	Source     string   `db:"source" json:"source"`
	Image      string   `db:"image" json:"image"`
	CatalogID  int64    `db:"catalog_id" json:"catalog_id"`

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

func (d *dao) GetCatalog(ctx context.Context, digest string) (*Catalog, error) {
	const query = `SELECT id, digest, name, source FROM catalog WHERE digest = $1`

	var catalog Catalog
	err := d.db.GetContext(ctx, &catalog, query, digest)
	if err != nil {
		return nil, err
	}

	const serverQuery = `SELECT id, server_type, tools, source, image, catalog_id, snapshot from catalog_server where catalog_id = $1`

	var servers []CatalogServer
	err = d.db.SelectContext(ctx, &servers, serverQuery, catalog.ID)
	if err != nil {
		return nil, err
	}
	catalog.Servers = servers

	return &catalog, nil
}

func (d *dao) CreateCatalog(ctx context.Context, catalog Catalog) error {
	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer txClose(tx, &err)

	const query = `INSERT INTO catalog (digest, name, source) VALUES ($1, $2, $3)`

	result, err := tx.ExecContext(ctx, query, catalog.Digest, catalog.Name, catalog.Source)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}

	for i := range catalog.Servers {
		catalog.Servers[i].CatalogID = id
	}

	if len(catalog.Servers) > 0 {
		const serverQuery = `INSERT INTO catalog_server (
		server_type, tools, source, image, catalog_id, snapshot
	) VALUES (:server_type, :tools, :source, :image, :catalog_id, :snapshot)`

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

func (d *dao) DeleteCatalog(ctx context.Context, digest string) error {
	const query = `DELETE FROM catalog WHERE digest = $1`

	_, err := d.db.ExecContext(ctx, query, digest)
	if err != nil {
		return err
	}
	return nil
}

func (d *dao) DeleteCatalogBySource(ctx context.Context, source string) error {
	if source == "" {
		return fmt.Errorf("source should not be empty when deleting a catalog")
	}

	const query = `DELETE FROM catalog WHERE source = $1`

	_, err := d.db.ExecContext(ctx, query, source)
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

	const query = `SELECT c.id, c.digest, c.name, c.source,
	COALESCE(
		json_group_array(json_object('id', s.id, 'server_type', s.server_type, 'tools', json(s.tools), 'source', s.source, 'image', s.image, 'snapshot', json(s.snapshot))),
		'[]'
	) AS server_json
	FROM catalog c
	LEFT JOIN catalog_server s ON s.catalog_id = c.id
	GROUP BY c.id`

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

func IsDuplicateDigestError(err error) bool {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE && strings.Contains(sqliteErr.Error(), "digest")
	}
	return false
}

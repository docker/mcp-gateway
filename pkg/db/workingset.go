package db

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"errors"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

type WorkingSetDAO interface {
	GetWorkingSet(ctx context.Context, id string) (*WorkingSet, error)
	FindWorkingSetsByIDPrefix(ctx context.Context, prefix string) ([]WorkingSet, error)
	ListWorkingSets(ctx context.Context) ([]WorkingSet, error)
	CreateWorkingSet(ctx context.Context, workingSet WorkingSet) error
	UpdateWorkingSet(ctx context.Context, workingSet WorkingSet) error
	RemoveWorkingSet(ctx context.Context, id string) error
	SearchWorkingSets(ctx context.Context, query string, workingSetID string) ([]WorkingSet, error)
}

type ServerList []Server

type SecretMap map[string]Secret

type WorkingSet struct {
	ID      string     `db:"id"`
	Name    string     `db:"name"`
	Servers ServerList `db:"servers"`
	Secrets SecretMap  `db:"secrets"`
}

type Server struct {
	Type     string         `json:"type"`
	Config   map[string]any `json:"config,omitempty"`
	Secrets  string         `json:"secrets,omitempty"`
	Tools    []string       `json:"tools"`
	Source   string         `json:"source,omitempty"`
	Image    string         `json:"image,omitempty"`
	Endpoint string         `json:"endpoint,omitempty"`

	// Optional snapshot of the server schema
	Snapshot *ServerSnapshot `json:"snapshot,omitempty"`
}

type Secret struct {
	Provider string `json:"provider"`
}

type ServerSnapshot struct {
	// TODO(cody): hacky reference to the same type that we use elsewhere
	Server catalog.Server `json:"server"`
}

// Used as a column in catalogs
func (snapshot *ServerSnapshot) Value() (driver.Value, error) {
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (snapshot *ServerSnapshot) Scan(value any) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("failed to scan server snapshot")
	}
	return json.Unmarshal([]byte(str), snapshot)
}

func (servers ServerList) Value() (driver.Value, error) {
	b, err := json.Marshal(servers)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (servers *ServerList) Scan(value any) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("failed to scan server list")
	}
	return json.Unmarshal([]byte(str), servers)
}

func (secrets SecretMap) Value() (driver.Value, error) {
	b, err := json.Marshal(secrets)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (secrets *SecretMap) Scan(value any) error {
	str, ok := value.(string)
	if !ok {
		return errors.New("failed to scan secret list")
	}
	return json.Unmarshal([]byte(str), secrets)
}

func (d *dao) GetWorkingSet(ctx context.Context, id string) (*WorkingSet, error) {
	const query = `SELECT id, name, servers, secrets FROM working_set WHERE id = $1`

	var workingSet WorkingSet
	err := d.db.GetContext(ctx, &workingSet, query, id)
	if err != nil {
		return nil, err
	}
	return &workingSet, nil
}

func (d *dao) RemoveWorkingSet(ctx context.Context, id string) error {
	const query = `DELETE FROM working_set WHERE id = $1`

	_, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}
	return nil
}

func (d *dao) CreateWorkingSet(ctx context.Context, workingSet WorkingSet) error {
	const query = `INSERT INTO working_set (id, name, servers, secrets) VALUES ($1, $2, $3, $4)`

	_, err := d.db.ExecContext(ctx, query, workingSet.ID, workingSet.Name, workingSet.Servers, workingSet.Secrets)
	if err != nil {
		return err
	}
	return nil
}

func (d *dao) UpdateWorkingSet(ctx context.Context, workingSet WorkingSet) error {
	const query = `UPDATE working_set SET name = $2, servers = $3, secrets = $4 WHERE id = $1`

	_, err := d.db.ExecContext(ctx, query, workingSet.ID, workingSet.Name, workingSet.Servers, workingSet.Secrets)
	if err != nil {
		return err
	}
	return nil
}

func (d *dao) FindWorkingSetsByIDPrefix(ctx context.Context, prefix string) ([]WorkingSet, error) {
	const query = `SELECT id, name, servers, secrets FROM working_set WHERE id LIKE $1`

	var workingSets []WorkingSet
	err := d.db.SelectContext(ctx, &workingSets, query, prefix+"%")
	if err != nil {
		return nil, err
	}
	return workingSets, nil
}

func (d *dao) ListWorkingSets(ctx context.Context) ([]WorkingSet, error) {
	const query = `SELECT id, name, servers, secrets FROM working_set`

	var workingSets []WorkingSet
	err := d.db.SelectContext(ctx, &workingSets, query)
	if err != nil {
		return nil, err
	}
	return workingSets, nil
}

func (d *dao) SearchWorkingSets(ctx context.Context, query string, workingSetID string) ([]WorkingSet, error) {
	sqlQuery := `
		SELECT id, name, servers, secrets
		FROM working_set
		WHERE ($1 = '' OR id = $1)
		  AND ($2 = '' OR EXISTS (
			SELECT 1
			FROM json_each(servers)
			WHERE LOWER(json_extract(value, '$.image')) LIKE '%' || LOWER($2) || '%'
			   OR LOWER(json_extract(value, '$.source')) LIKE '%' || LOWER($2) || '%'
		  ))
		ORDER BY id
	`
	args := []any{workingSetID, query}

	var workingSets []WorkingSet
	err := d.db.SelectContext(ctx, &workingSets, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	return workingSets, nil
}

package db

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
)

type WorkingSetDAO interface {
	GetWorkingSet(ctx context.Context, id string) (*WorkingSet, error)
	CreateWorkingSet(ctx context.Context, workingSet WorkingSet) error
	UpdateWorkingSet(ctx context.Context, workingSet WorkingSet) error
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
	Type    string                 `json:"type"`
	Config  map[string]interface{} `json:"config,omitempty"`
	Secrets string                 `json:"secrets,omitempty"`
	Tools   []string               `json:"tools,omitempty"`
	Source  string                 `json:"source,omitempty"`
	Image   string                 `json:"image,omitempty"`
}

type Secret struct {
	Provider string `json:"provider"`
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
	return json.Unmarshal([]byte(str), &servers)
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
	return json.Unmarshal([]byte(str), &secrets)
}

func (d *Dao) GetWorkingSet(ctx context.Context, id string) (*WorkingSet, error) {
	const query = `SELECT id, name, servers, secrets FROM working_set WHERE id = $1`

	var workingSet WorkingSet
	err := d.db.GetContext(ctx, &workingSet, query, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &workingSet, nil
}

func (d *Dao) CreateWorkingSet(ctx context.Context, workingSet WorkingSet) error {
	const query = `INSERT INTO working_set (id, name, servers, secrets) VALUES ($1, $2, $3, $4)`

	_, err := d.db.ExecContext(ctx, query, workingSet.ID, workingSet.Name, workingSet.Servers, workingSet.Secrets)
	if err != nil {
		return err
	}
	return nil
}

func (d *Dao) UpdateWorkingSet(ctx context.Context, workingSet WorkingSet) error {
	const query = `UPDATE working_set SET name = $2, servers = $3, secrets = $4 WHERE id = $1`

	_, err := d.db.ExecContext(ctx, query, workingSet.ID, workingSet.Name, workingSet.Servers, workingSet.Secrets)
	if err != nil {
		return err
	}
	return nil
}

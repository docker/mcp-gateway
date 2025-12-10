package gateway

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/docker/mcp-gateway/pkg/db"
	"github.com/docker/mcp-gateway/pkg/oci"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

var ErrWorkingSetNotFound = errors.New("profile not found")

type WorkingSetLoader interface {
	ReadWorkingSet(ctx context.Context) (workingset.WorkingSet, error)
}

type databaseLoader struct {
	workingSet string
	dao        db.DAO
}

func NewWorkingSetDatabaseLoader(workingSet string, dao db.DAO) WorkingSetLoader {
	return &databaseLoader{workingSet: workingSet, dao: dao}
}

func (l *databaseLoader) ReadWorkingSet(ctx context.Context) (workingset.WorkingSet, error) {
	workingSet, err := l.dao.GetWorkingSet(ctx, l.workingSet)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return workingset.WorkingSet{}, fmt.Errorf("%w: %s", ErrWorkingSetNotFound, l.workingSet)
		}
		return workingset.WorkingSet{}, err
	}
	return workingset.NewFromDb(workingSet), nil
}

type fileLoader struct {
	workingSetFile string
	ociService     oci.Service
}

func NewWorkingSetFileLoader(workingSetFile string, ociService oci.Service) WorkingSetLoader {
	return &fileLoader{workingSetFile: workingSetFile, ociService: ociService}
}

func (l *fileLoader) ReadWorkingSet(ctx context.Context) (workingset.WorkingSet, error) {
	workingSet, err := workingset.ReadFromFile(ctx, l.ociService, l.workingSetFile)
	if err != nil {
		if os.IsNotExist(err) {
			return workingset.WorkingSet{}, fmt.Errorf("%w: %s", ErrWorkingSetNotFound, l.workingSetFile)
		}
		return workingset.WorkingSet{}, err
	}

	if err := workingSet.Validate(); err != nil {
		return workingset.WorkingSet{}, fmt.Errorf("invalid profile: %w", err)
	}

	return workingSet, nil
}

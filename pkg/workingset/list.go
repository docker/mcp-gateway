package workingset

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/pkg/db"
)

func List(ctx context.Context, dao db.DAO) error {
	dbSets, err := dao.ListWorkingSets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list working sets: %w", err)
	}

	workingSets := make([]WorkingSet, len(dbSets))
	for i, dbWorkingSet := range dbSets {
		workingSets[i] = NewFromDb(&dbWorkingSet)
	}

	fmt.Println("ID\tName")
	fmt.Println("----\t----")
	for _, workingSet := range workingSets {
		fmt.Printf("%s\t%s\n", workingSet.ID, workingSet.Name)
	}
	fmt.Println()

	return nil
}

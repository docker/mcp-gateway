package policy

import (
	"context"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/desktop"
)

func Dump(ctx context.Context) error {
	l, err := desktop.NewSecretsClient().GetJfsPolicy(ctx)
	if err != nil {
		return err
	}

	fmt.Println(l)
	return nil
}

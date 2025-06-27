package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

type info struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func getRegistryAuth(ctx context.Context) (string, error) {
	// This call forces the refresh of the token. `/registry/info`` fails to do it sometimes.
	var token string
	if err := desktop.ClientBackend.Get(ctx, "/registry/token", &token); err != nil {
		return "", nil
	}

	var info info
	if err := desktop.ClientBackend.Get(ctx, "/registry/info", &info); err != nil {
		return "", nil
	}

	authConfig := map[string]string{
		"username": info.ID,
		"password": token,
	}
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshalling auth config: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

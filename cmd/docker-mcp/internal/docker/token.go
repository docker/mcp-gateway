package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/desktop"
)

type info struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func getRegistryAuth(ctx context.Context) (string, error) {
	var info info
	if err := desktop.ClientBackend.Get(ctx, "/registry/info", &info); err != nil {
		log("warning: couldn't read the auth token:", err)
		return "", nil
	}

	authConfig := map[string]string{
		"username": info.ID,
		"password": info.Token,
	}
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("marshalling auth config: %w", err)
	}

	return base64.StdEncoding.EncodeToString(buf), nil
}

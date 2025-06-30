package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

type info struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

func getRegistryAuth(ctx context.Context) string {
	// This call forces the refresh of the token. `/registry/info`` fails to do it sometimes.
	var token string
	if err := desktop.ClientBackend.Get(ctx, "/registry/token", &token); err != nil {
		return ""
	}

	var info info
	if err := desktop.ClientBackend.Get(ctx, "/registry/info", &info); err != nil {
		return ""
	}

	authConfig := map[string]string{
		"username": info.ID,
		"password": token,
	}
	buf, err := json.Marshal(authConfig)
	if err != nil {
		return ""
	}

	return base64.StdEncoding.EncodeToString(buf)
}

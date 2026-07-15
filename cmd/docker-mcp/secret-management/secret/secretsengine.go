package secret

import (
	"context"
	"errors"

	"github.com/docker/secrets-engine/client"
	"github.com/docker/secrets-engine/client/realms"
	"github.com/docker/secrets-engine/x/api"
)

// ErrSecretNotFound is returned when a requested secret does not exist.
// It aliases the SDK's not-found error so callers can use errors.Is against either.
var ErrSecretNotFound = client.ErrSecretNotFound

// newClient builds a Secrets Engine client pinned to the engine socket.
func newClient() (client.Client, error) {
	return client.New(client.WithSocketPath(api.DefaultSocketPath()))
}

// GetSecrets returns all secrets under the docker/mcp/** realm.
func GetSecrets(ctx context.Context) ([]client.Envelope, error) {
	c, err := newClient()
	if err != nil {
		return nil, err
	}

	envelopes, err := c.GetSecrets(ctx, realms.DockerMCPDefault)
	if errors.Is(err, ErrSecretNotFound) {
		return []client.Envelope{}, nil
	}
	if err != nil {
		return nil, err
	}
	return envelopes, nil
}

// GetSecret retrieves a single secret by its full ID (e.g., "docker/mcp/oauth/github").
// Returns ErrSecretNotFound if the secret does not exist.
func GetSecret(ctx context.Context, id client.ID) (*client.Envelope, error) {
	pattern, err := client.ParsePattern(id.String())
	if err != nil {
		return nil, err
	}

	c, err := newClient()
	if err != nil {
		return nil, err
	}

	envelopes, err := c.GetSecrets(ctx, pattern)
	if errors.Is(err, ErrSecretNotFound) {
		return nil, ErrSecretNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(envelopes) == 0 {
		return nil, ErrSecretNotFound
	}
	return &envelopes[0], nil
}

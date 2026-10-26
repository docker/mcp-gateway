package secret

import (
	"context"
	"errors"
	"fmt"

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
	envelopes, err := getSecretsByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return &envelopes[0], nil
}

// GetSecretFromProvider retrieves a secret by ID from one specific Secrets
// Engine provider when multiple providers resolve the same realm.
func GetSecretFromProvider(ctx context.Context, id client.ID, provider string) (*client.Envelope, error) {
	envelopes, err := getSecretsByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return secretFromProvider(envelopes, id, provider)
}

func secretFromProvider(envelopes []client.Envelope, id client.ID, provider string) (*client.Envelope, error) {
	for i := range envelopes {
		if envelopes[i].Provider == provider {
			return &envelopes[i], nil
		}
	}
	return nil, fmt.Errorf("%w: provider %q for %s", ErrSecretNotFound, provider, id.String())
}

func getSecretsByID(ctx context.Context, id client.ID) ([]client.Envelope, error) {
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
	return envelopes, nil
}

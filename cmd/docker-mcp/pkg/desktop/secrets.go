package desktop

import (
	"context"
	"fmt"
)

type StoredSecret struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
}

type Secret struct {
	Name     string `json:"name"`
	Provider string `json:"provider,omitempty"`
	Value    string `json:"value"`
}

func NewSecretsClient() *Secrets {
	return &Secrets{
		rawClient: newRawClient(dialSecrets),
	}
}

type Secrets struct {
	rawClient *RawClient
}

func (c *Secrets) DeleteJfsSecret(ctx context.Context, secret string) error {
	AvoidResourceSaverMode(ctx)

	return c.rawClient.Delete(ctx, fmt.Sprintf("/secrets/%v", secret))
}

func (c *Secrets) GetJfsPolicy(ctx context.Context) (string, error) {
	AvoidResourceSaverMode(ctx)

	var result string
	err := c.rawClient.Get(ctx, "/policy", &result)
	return result, err
}

func (c *Secrets) ListJfsSecrets(ctx context.Context) ([]StoredSecret, error) {
	AvoidResourceSaverMode(ctx)

	var result []StoredSecret
	err := c.rawClient.Get(ctx, "/secrets", &result)
	return result, err
}

func (c *Secrets) SetJfsPolicy(ctx context.Context, body string) error {
	AvoidResourceSaverMode(ctx)

	return c.rawClient.Post(ctx, "/policy", body, nil)
}

func (c *Secrets) SetJfsSecret(ctx context.Context, secret Secret) error {
	AvoidResourceSaverMode(ctx)

	return c.rawClient.Post(ctx, "/secrets", secret, nil)
}

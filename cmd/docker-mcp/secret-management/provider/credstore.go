package provider

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

// CredStoreProvider implements SecretProvider using Docker credential helpers.
type CredStoreProvider struct {
	helper credentials.Helper
}

// NewCredStoreProvider creates a new CredStoreProvider.
func NewCredStoreProvider() *CredStoreProvider {
	return &CredStoreProvider{
		helper: getHelper(),
	}
}

// GetSecret retrieves a secret from the credential store.
func (c *CredStoreProvider) GetSecret(ctx context.Context, name string) (string, error) {
	_, value, err := c.helper.Get(getSecretKey(name))
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return "", &SecretNotFoundError{Name: name, Provider: "credstore"}
		}
		return "", err
	}
	
	return value, nil
}

// SetSecret stores a secret in the credential store.
func (c *CredStoreProvider) SetSecret(ctx context.Context, name, value string) error {
	creds := &credentials.Credentials{
		ServerURL: getSecretKey(name),
		Username:  "mcp",
		Secret:    value,
	}
	
	return c.helper.Add(creds)
}

// DeleteSecret removes a secret from the credential store.
func (c *CredStoreProvider) DeleteSecret(ctx context.Context, name string) error {
	err := c.helper.Delete(getSecretKey(name))
	if err != nil && credentials.IsErrCredentialsNotFound(err) {
		return &SecretNotFoundError{Name: name, Provider: "credstore"}
	}
	return err
}

// ListSecrets returns all secrets from the credential store.
func (c *CredStoreProvider) ListSecrets(ctx context.Context) ([]StoredSecret, error) {
	allCreds, err := c.helper.List()
	if err != nil {
		return nil, err
	}
	
	var secrets []StoredSecret
	for serverURL := range allCreds {
		if strings.HasPrefix(serverURL, "sm_") {
			name := strings.TrimPrefix(serverURL, "sm_")
			secrets = append(secrets, StoredSecret{
				Name:     name,
				Provider: "credstore",
			})
		}
	}
	
	return secrets, nil
}

// IsAvailable checks if the credential store is available.
func (c *CredStoreProvider) IsAvailable(ctx context.Context) bool {
	// Check if docker-credential-pass is available
	_, err := exec.LookPath("docker-credential-pass")
	return err == nil
}

// ProviderName returns the name of this provider.
func (c *CredStoreProvider) ProviderName() string {
	return "credstore"
}

// SupportsSecureMount retorna false - Credstore precisa expor valores
func (c *CredStoreProvider) SupportsSecureMount() bool {
	return false
}

// getSecretKey returns the key used to store secrets in the credential store.
func getSecretKey(secretName string) string {
	return "sm_" + secretName
}

// getHelper returns a credential helper instance.
func getHelper() credentials.Helper {
	credentialHelperPath := desktop.Paths().CredentialHelperPath()
	return Helper{
		program: newShellProgramFunc(credentialHelperPath),
	}
}

// newShellProgramFunc creates programs that are executed in a Shell.
func newShellProgramFunc(name string) client.ProgramFunc {
	return func(args ...string) client.Program {
		return &shell{cmd: exec.CommandContext(context.Background(), name, args...)}
	}
}

// shell invokes shell commands to talk with a remote credentials-helper.
type shell struct {
	cmd *exec.Cmd
}

// Output returns responses from the remote credentials-helper.
func (s *shell) Output() ([]byte, error) {
	return s.cmd.Output()
}

// Input sets the input to send to a remote credentials-helper.
func (s *shell) Input(in io.Reader) {
	s.cmd.Stdin = in
}

// Helper wraps credential helper program.
type Helper struct {
	program client.ProgramFunc
}

func (h Helper) List() (map[string]string, error) {
	return map[string]string{}, nil
}

// Add stores new credentials.
func (h Helper) Add(creds *credentials.Credentials) error {
	username, secret, err := h.Get(creds.ServerURL)
	if err != nil && !credentials.IsErrCredentialsNotFound(err) && !isErrDecryption(err) {
		return err
	}
	if username == creds.Username && secret == creds.Secret {
		return nil
	}
	if err := client.Store(h.program, creds); err != nil {
		return err
	}
	return nil
}

// Delete removes credentials.
func (h Helper) Delete(serverURL string) error {
	if _, _, err := h.Get(serverURL); err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			return nil
		}
		return err
	}
	return client.Erase(h.program, serverURL)
}

// Get returns the username and secret to use for a given registry server URL.
func (h Helper) Get(serverURL string) (string, string, error) {
	creds, err := client.Get(h.program, serverURL)
	if err != nil {
		return "", "", err
	}
	return creds.Username, creds.Secret, nil
}

func isErrDecryption(err error) bool {
	return err != nil && strings.Contains(err.Error(), "gpg: decryption failed: No secret key")
}

var _ credentials.Helper = Helper{}
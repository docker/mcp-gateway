package secret

import (
	"context"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/pkg/desktop"
)

type CredStoreProvider struct {
	credentialHelper credentials.Helper
}

func NewCredStoreProvider() *CredStoreProvider {
	return &CredStoreProvider{credentialHelper: GetHelper()}
}

func getSecretKey(secretName string) string {
	return "sm_" + secretName
}

func (store *CredStoreProvider) GetSecret(id string) (string, error) {
	_, val, err := store.credentialHelper.Get(getSecretKey(id))
	if err != nil {
		return "", err
	}
	return val, nil
}

func (store *CredStoreProvider) SetSecret(id string, value string) error {
	return store.credentialHelper.Add(&credentials.Credentials{
		ServerURL: getSecretKey(id),
		Username:  "mcp",
		Secret:    value,
	})
}

func (store *CredStoreProvider) DeleteSecret(id string) error {
	return store.credentialHelper.Delete(getSecretKey(id))
}

func GetHelper() credentials.Helper {
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
	// name    string
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

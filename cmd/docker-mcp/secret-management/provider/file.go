package provider

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/docker/mcp-gateway/pkg/user"
)

// FileProvider implements SecretProvider using local file storage.
type FileProvider struct {
	secretsDir string
	secretsFile string
	encryptionKey []byte
}

// FileSecretData represents the structure of the secrets file.
type FileSecretData struct {
	Secrets map[string]EncryptedSecret `json:"secrets"`
}

// EncryptedSecret represents an encrypted secret.
type EncryptedSecret struct {
	Data  string `json:"data"`  // Base64 encoded encrypted data
	Nonce string `json:"nonce"` // Base64 encoded nonce for AES-GCM
}

// NewFileProvider creates a new FileProvider.
func NewFileProvider() *FileProvider {
	homeDir, err := user.HomeDir()
	if err != nil {
		homeDir = os.TempDir()
	}
	
	secretsDir := filepath.Join(homeDir, ".docker", "mcp", "secrets")
	secretsFile := filepath.Join(secretsDir, "secrets.json")
	
	// Generate or load encryption key
	key, err := getOrCreateEncryptionKey(secretsDir)
	if err != nil {
		// Fall back to a deterministic key if we can't create one
		// This is not ideal for security but ensures functionality
		hash := sha256.Sum256([]byte("mcp-gateway-fallback-key"))
		key = hash[:]
	}
	
	return &FileProvider{
		secretsDir:    secretsDir,
		secretsFile:   secretsFile,
		encryptionKey: key,
	}
}

// GetSecret retrieves a secret from the file.
func (f *FileProvider) GetSecret(ctx context.Context, name string) (string, error) {
	data, err := f.loadSecretsData()
	if err != nil {
		return "", err
	}
	
	encrypted, exists := data.Secrets[name]
	if !exists {
		return "", &SecretNotFoundError{Name: name, Provider: "file"}
	}
	
	value, err := f.decrypt(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret %s: %w", name, err)
	}
	
	return value, nil
}

// SetSecret stores a secret in the file.
func (f *FileProvider) SetSecret(ctx context.Context, name, value string) error {
	if err := f.ensureSecretsDir(); err != nil {
		return err
	}
	
	data, err := f.loadSecretsData()
	if err != nil {
		data = &FileSecretData{Secrets: make(map[string]EncryptedSecret)}
	}
	
	encrypted, err := f.encrypt(value)
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}
	
	data.Secrets[name] = encrypted
	
	return f.saveSecretsData(data)
}

// DeleteSecret removes a secret from the file.
func (f *FileProvider) DeleteSecret(ctx context.Context, name string) error {
	data, err := f.loadSecretsData()
	if err != nil {
		return err
	}
	
	if _, exists := data.Secrets[name]; !exists {
		return &SecretNotFoundError{Name: name, Provider: "file"}
	}
	
	delete(data.Secrets, name)
	
	return f.saveSecretsData(data)
}

// ListSecrets returns all secrets from the file.
func (f *FileProvider) ListSecrets(ctx context.Context) ([]StoredSecret, error) {
	data, err := f.loadSecretsData()
	if err != nil {
		return nil, err
	}
	
	var secrets []StoredSecret
	for name := range data.Secrets {
		secrets = append(secrets, StoredSecret{
			Name:     name,
			Provider: "file",
		})
	}
	
	return secrets, nil
}

// IsAvailable checks if file storage is available.
func (f *FileProvider) IsAvailable(ctx context.Context) bool {
	// File storage is always available as a fallback
	return true
}

// ProviderName returns the name of this provider.
func (f *FileProvider) ProviderName() string {
	return "file"
}

// SupportsSecureMount returns false - File provider needs to expose values
func (f *FileProvider) SupportsSecureMount() bool {
	return false
}

// ensureSecretsDir creates the secrets directory if it doesn't exist.
func (f *FileProvider) ensureSecretsDir() error {
	return os.MkdirAll(f.secretsDir, 0700)
}

// loadSecretsData loads and parses the secrets file.
func (f *FileProvider) loadSecretsData() (*FileSecretData, error) {
	data, err := os.ReadFile(f.secretsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileSecretData{Secrets: make(map[string]EncryptedSecret)}, nil
		}
		return nil, err
	}
	
	var secretsData FileSecretData
	if err := json.Unmarshal(data, &secretsData); err != nil {
		return nil, fmt.Errorf("failed to parse secrets file: %w", err)
	}
	
	if secretsData.Secrets == nil {
		secretsData.Secrets = make(map[string]EncryptedSecret)
	}
	
	return &secretsData, nil
}

// saveSecretsData saves the secrets data to file.
func (f *FileProvider) saveSecretsData(data *FileSecretData) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	
	return os.WriteFile(f.secretsFile, jsonData, 0600)
}

// encrypt encrypts a value using AES-GCM.
func (f *FileProvider) encrypt(value string) (EncryptedSecret, error) {
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return EncryptedSecret{}, err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return EncryptedSecret{}, err
	}
	
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return EncryptedSecret{}, err
	}
	
	ciphertext := gcm.Seal(nil, nonce, []byte(value), nil)
	
	return EncryptedSecret{
		Data:  base64.StdEncoding.EncodeToString(ciphertext),
		Nonce: base64.StdEncoding.EncodeToString(nonce),
	}, nil
}

// decrypt decrypts a value using AES-GCM.
func (f *FileProvider) decrypt(encrypted EncryptedSecret) (string, error) {
	block, err := aes.NewCipher(f.encryptionKey)
	if err != nil {
		return "", err
	}
	
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted.Data)
	if err != nil {
		return "", err
	}
	
	nonce, err := base64.StdEncoding.DecodeString(encrypted.Nonce)
	if err != nil {
		return "", err
	}
	
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	
	return string(plaintext), nil
}

// getOrCreateEncryptionKey generates or loads an encryption key.
func getOrCreateEncryptionKey(secretsDir string) ([]byte, error) {
	keyFile := filepath.Join(secretsDir, ".key")
	
	// Try to load existing key
	if data, err := os.ReadFile(keyFile); err == nil {
		if len(data) == 32 {
			return data, nil
		}
	}
	
	// Generate new key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	
	// Ensure directory exists
	if err := os.MkdirAll(secretsDir, 0700); err != nil {
		return nil, err
	}
	
	// Save key
	if err := os.WriteFile(keyFile, key, 0600); err != nil {
		return nil, err
	}
	
	return key, nil
}
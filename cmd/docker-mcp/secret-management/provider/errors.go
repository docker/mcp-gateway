package provider

import "fmt"

// SecretNotFoundError is returned when a secret is not found in a provider.
type SecretNotFoundError struct {
	Name     string
	Provider string
}

// Error implements the error interface.
func (e *SecretNotFoundError) Error() string {
	return fmt.Sprintf("secret %q not found in provider %q", e.Name, e.Provider)
}

// IsSecretNotFound checks if an error is a SecretNotFoundError.
func IsSecretNotFound(err error) bool {
	_, ok := err.(*SecretNotFoundError)
	return ok
}
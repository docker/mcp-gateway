package provider

import (
	"context"
	"path/filepath"
	"testing"
)

func TestFileProvider(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	
	// Create a file provider with custom directory
	provider := &FileProvider{
		secretsDir:    tempDir,
		secretsFile:   filepath.Join(tempDir, "secrets.json"),
		encryptionKey: []byte("0123456789abcdef0123456789abcdef"), // 32 bytes for AES-256
	}
	
	ctx := context.Background()
	
	// Test that the provider is available
	if !provider.IsAvailable(ctx) {
		t.Fatal("File provider should always be available")
	}
	
	// Test setting a secret
	err := provider.SetSecret(ctx, "test_key", "test_value")
	if err != nil {
		t.Fatalf("Failed to set secret: %v", err)
	}
	
	// Test getting the secret
	value, err := provider.GetSecret(ctx, "test_key")
	if err != nil {
		t.Fatalf("Failed to get secret: %v", err)
	}
	
	if value != "test_value" {
		t.Fatalf("Expected 'test_value', got '%s'", value)
	}
	
	// Test listing secrets
	secrets, err := provider.ListSecrets(ctx)
	if err != nil {
		t.Fatalf("Failed to list secrets: %v", err)
	}
	
	if len(secrets) != 1 {
		t.Fatalf("Expected 1 secret, got %d", len(secrets))
	}
	
	if secrets[0].Name != "test_key" {
		t.Fatalf("Expected secret name 'test_key', got '%s'", secrets[0].Name)
	}
	
	if secrets[0].Provider != "file" {
		t.Fatalf("Expected provider 'file', got '%s'", secrets[0].Provider)
	}
	
	// Test getting non-existent secret
	_, err = provider.GetSecret(ctx, "non_existent")
	if err == nil {
		t.Fatal("Expected error for non-existent secret")
	}
	
	if !IsSecretNotFound(err) {
		t.Fatalf("Expected SecretNotFoundError, got %T", err)
	}
	
	// Test deleting the secret
	err = provider.DeleteSecret(ctx, "test_key")
	if err != nil {
		t.Fatalf("Failed to delete secret: %v", err)
	}
	
	// Verify secret is gone
	_, err = provider.GetSecret(ctx, "test_key")
	if err == nil {
		t.Fatal("Expected error after deleting secret")
	}
	
	// Test deleting non-existent secret
	err = provider.DeleteSecret(ctx, "non_existent")
	if err == nil {
		t.Fatal("Expected error for deleting non-existent secret")
	}
	
	if !IsSecretNotFound(err) {
		t.Fatalf("Expected SecretNotFoundError, got %T", err)
	}
}

func TestChainProvider(t *testing.T) {
	ctx := context.Background()
	
	// Create two file providers with different directories
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()
	
	provider1 := &FileProvider{
		secretsDir:    tempDir1,
		secretsFile:   filepath.Join(tempDir1, "secrets.json"),
		encryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	}
	
	provider2 := &FileProvider{
		secretsDir:    tempDir2,
		secretsFile:   filepath.Join(tempDir2, "secrets.json"),
		encryptionKey: []byte("fedcba9876543210fedcba9876543210"),
	}
	
	// Add a secret to provider2
	err := provider2.SetSecret(ctx, "secret_in_provider2", "value2")
	if err != nil {
		t.Fatalf("Failed to set secret in provider2: %v", err)
	}
	
	// Create a chain provider
	chain := NewChainProvider(provider1, provider2)
	
	// Test that chain is available
	if !chain.IsAvailable(ctx) {
		t.Fatal("Chain provider should be available")
	}
	
	// Test setting a secret (should go to first provider)
	err = chain.SetSecret(ctx, "chain_secret", "chain_value")
	if err != nil {
		t.Fatalf("Failed to set secret in chain: %v", err)
	}
	
	// Verify secret is in provider1
	value, err := provider1.GetSecret(ctx, "chain_secret")
	if err != nil {
		t.Fatalf("Secret not found in provider1: %v", err)
	}
	if value != "chain_value" {
		t.Fatalf("Expected 'chain_value', got '%s'", value)
	}
	
	// Test getting secret from chain (should find in provider1)
	value, err = chain.GetSecret(ctx, "chain_secret")
	if err != nil {
		t.Fatalf("Failed to get secret from chain: %v", err)
	}
	if value != "chain_value" {
		t.Fatalf("Expected 'chain_value', got '%s'", value)
	}
	
	// Test getting secret that exists only in provider2
	value, err = chain.GetSecret(ctx, "secret_in_provider2")
	if err != nil {
		t.Fatalf("Failed to get secret from provider2 via chain: %v", err)
	}
	if value != "value2" {
		t.Fatalf("Expected 'value2', got '%s'", value)
	}
	
	// Test listing secrets from chain
	secrets, err := chain.ListSecrets(ctx)
	if err != nil {
		t.Fatalf("Failed to list secrets from chain: %v", err)
	}
	
	// Should have both secrets (one from each provider)
	if len(secrets) != 2 {
		t.Fatalf("Expected 2 secrets, got %d", len(secrets))
	}
	
	// Test deleting secret from chain
	err = chain.DeleteSecret(ctx, "chain_secret")
	if err != nil {
		t.Fatalf("Failed to delete secret from chain: %v", err)
	}
	
	// Verify secret is gone from provider1
	_, err = provider1.GetSecret(ctx, "chain_secret")
	if err == nil {
		t.Fatal("Secret should be deleted from provider1")
	}
}

func TestEncryptionDecryption(t *testing.T) {
	provider := &FileProvider{
		encryptionKey: []byte("0123456789abcdef0123456789abcdef"),
	}
	
	testValue := "sensitive_data_123!@#"
	
	// Test encryption
	encrypted, err := provider.encrypt(testValue)
	if err != nil {
		t.Fatalf("Failed to encrypt: %v", err)
	}
	
	// Verify encrypted data is different from original
	if encrypted.Data == testValue {
		t.Fatal("Encrypted data should not match original")
	}
	
	// Test decryption
	decrypted, err := provider.decrypt(encrypted)
	if err != nil {
		t.Fatalf("Failed to decrypt: %v", err)
	}
	
	// Verify decrypted data matches original
	if decrypted != testValue {
		t.Fatalf("Expected '%s', got '%s'", testValue, decrypted)
	}
}
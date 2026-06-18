package mcpsecret

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/sts/types"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/secret-management/secret"
)

// AWSCredentials represents temporary AWS STS credentials stored as JSON.
type AWSCredentials struct {
	AccessKeyId     string    `json:"accessKeyId"`
	SecretAccessKey string    `json:"secretAccessKey"`
	SessionToken    string    `json:"sessionToken"`
	Expiration      time.Time `json:"expiration"`
}

// AWSManager handles generation and storage of temporary STS credentials in the MCP credstore.
type AWSManager struct{}

// NewAWSManager creates a new instance of AWSManager.
func NewAWSManager() *AWSManager {
	return &AWSManager{}
}

// SaveTemporaryCredentials generates temporary AWS STS credentials and stores them securely
// in the MCP credstore as a JSON-encoded AWSCredentials object.
//
// If roleARN is empty (""), it uses GetSessionToken (for IAM users with direct access).
// If roleARN is provided, it uses AssumeRole (recommended for production / cross-account scenarios).
//
// roleSessionName is optional when using AssumeRole; defaults to "mcp-gateway-<serverName>".
// durationSeconds defaults to 3600 if <= 0; must be between 900 and 129600 seconds (STS limits).
//
// Retries transient failures on both STS calls and MCP writes.
// All operations respect context cancellation.
func (m *AWSManager) SaveTemporaryCredentials(ctx context.Context, serverName string, durationSeconds int32, roleARN, roleSessionName string) error {
	if serverName == "" {
		return errors.New("serverName cannot be empty")
	}

	// Sanitize serverName to prevent invalid secret keys
	serverName = strings.TrimSpace(serverName)
	serverName = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ' ', ':', '.', '*':
			return '_'
		default:
			return r
		}
	}, serverName)

	// Handle duration default and validation (STS enforced limits)
	if durationSeconds <= 0 {
		durationSeconds = 3600
	}
	if durationSeconds < 900 || durationSeconds > 129600 {
		return fmt.Errorf("durationSeconds must be between 900 and 129600 seconds (got %d)", durationSeconds)
	}

	// Load AWS config once (shared for both paths)
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS config: %w", err)
	}
	stsClient := sts.NewFromConfig(cfg)

	// Prepare to hold the resulting credentials
	var stsCreds *types.Credentials
	var stsErr error
	const maxAttempts = 3

	// --- STS credential retrieval with retries ---
	if roleARN != "" {
		// AssumeRole path (most common in enterprise setups)
		if roleSessionName == "" {
			roleSessionName = fmt.Sprintf("mcp-gateway-%s", serverName)
		}

		input := &sts.AssumeRoleInput{
			RoleArn:         aws.String(roleARN),
			RoleSessionName: aws.String(roleSessionName),
			DurationSeconds: aws.Int32(durationSeconds),
		}

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			output, err := stsClient.AssumeRole(ctx, input)
			if err == nil {
				stsCreds = output.Credentials
				break
			}
			stsErr = err
			log.Printf("[AWS STS AssumeRole] attempt %d/%d failed for role %s: %v", attempt, maxAttempts, roleARN, err)
			if ctx.Err() != nil {
				return fmt.Errorf("operation canceled during AssumeRole: %w", ctx.Err())
			}
			time.Sleep(time.Duration(attempt*300) * time.Millisecond)
		}
	} else {
		// GetSessionToken path (direct IAM user credentials)
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			output, err := stsClient.GetSessionToken(ctx, &sts.GetSessionTokenInput{
				DurationSeconds: aws.Int32(durationSeconds),
			})
			if err == nil {
				stsCreds = output.Credentials
				break
			}
			stsErr = err
			log.Printf("[AWS STS GetSessionToken] attempt %d/%d failed: %v", attempt, maxAttempts, err)
			if ctx.Err() != nil {
				return fmt.Errorf("operation canceled during GetSessionToken: %w", ctx.Err())
			}
			time.Sleep(time.Duration(attempt*300) * time.Millisecond)
		}
	}

	if stsErr != nil {
		return fmt.Errorf("failed to obtain STS credentials after %d attempts: %w", maxAttempts, stsErr)
	}

	// Validate the received credentials (both paths produce the same *types.Credentials)
	if stsCreds == nil || stsCreds.AccessKeyId == nil || stsCreds.SecretAccessKey == nil ||
		stsCreds.SessionToken == nil || stsCreds.Expiration.IsZero() {
		return errors.New("received invalid or incomplete STS credentials")
	}

	// Prepare JSON payload
	credObj := AWSCredentials{
		AccessKeyId:     *stsCreds.AccessKeyId,
		SecretAccessKey: *stsCreds.SecretAccessKey,
		SessionToken:    *stsCreds.SessionToken,
		Expiration:      stsCreds.Expiration,
	}

	jsonBytes, err := json.Marshal(credObj)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials to JSON: %w", err)
	}

	// Prepare MCP secret write
	opts := secret.SetOpts{Provider: secret.Credstore}
	if !secret.IsValidProvider(opts.Provider) {
		return fmt.Errorf("invalid secret provider: %s", opts.Provider)
	}

	secretKey := fmt.Sprintf("aws/sts/%s", serverName)
	mainSecret := secret.Secret{Key: secretKey, Val: string(jsonBytes)}

	// Retry loop for storing in MCP credstore
	var writeErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		writeErr = secret.Set(ctx, mainSecret, opts)
		if writeErr == nil {
			break
		}
		log.Printf("[MCP Credstore] write attempt %d/%d for key %s failed: %v", attempt, maxAttempts, secretKey, writeErr)
		if ctx.Err() != nil {
			return fmt.Errorf("MCP write canceled: %w", ctx.Err())
		}
		time.Sleep(time.Duration(attempt*200) * time.Millisecond)
	}

	if writeErr != nil {
		return fmt.Errorf("failed to store credentials in MCP credstore after %d attempts: %w", maxAttempts, writeErr)
	}

	log.Printf("[AWSManager] Successfully stored STS credentials for server %s (expires: %s) via %s",
		serverName,
		stsCreds.Expiration.Format(time.RFC3339),
		func() string {
			if roleARN != "" {
				return "AssumeRole"
			}
			return "GetSessionToken"
		}())

	return nil
}

// DescribeSecret returns a human-readable summary of the AWS STS credentials
// without exposing sensitive values. Only the AccessKeyId is partially visible.
// The secretValue is expected to be a JSON-encoded AWSCredentials object.
func (m *AWSManager) DescribeSecret(serverName, secretValue string) string {
	// Validate server name input
	if serverName == "" {
		return "Invalid server name"
	}

	// Attempt to parse the secret JSON into AWSCredentials struct
	var credObj AWSCredentials
	if err := json.Unmarshal([]byte(secretValue), &credObj); err != nil {
		// Return a user-friendly message if the JSON is invalid or corrupted
		return fmt.Sprintf("Invalid or corrupted credentials for server %s", serverName)
	}

	// Check if AccessKeyId is present
	if credObj.AccessKeyId == "" {
		return fmt.Sprintf("No valid AccessKeyId for server %s", serverName)
	}

	// Determine the expiration status of the credentials
	now := time.Now()
	var expDesc string
	switch {
	case credObj.Expiration.IsZero():
		// Expiration field is missing or zero
		expDesc = "expiration unknown"
	case credObj.Expiration.Before(now):
		// Credentials have already expired
		expDesc = fmt.Sprintf("expired since %s", credObj.Expiration.Format(time.RFC3339))
	default:
		// Credentials are still valid
		expDesc = fmt.Sprintf("expires at %s", credObj.Expiration.Format(time.RFC3339))
	}

	// Return a formatted, human-readable string
	// The AccessKeyId is partially masked for security
	return fmt.Sprintf(
		"AWS STS credentials for %s: AccessKeyID=%s (%s)",
		serverName,
		maskString(credObj.AccessKeyId),
		expDesc,
	)
}

// maskString masks all but the last 4 characters of a string.
// Returns "****" if the input is empty or too short.
func maskString(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= 4 {
		return "****"
	}
	return s[:len(s)-4] + "****"
}

// LoadCredentials retrieves AWS STS credentials for a given server from MCP.
// Returns an error if the credentials are missing, invalid, or expired.
func (m *AWSManager) LoadCredentials(ctx context.Context, serverName string) (*AWSCredentials, error) {
	if serverName == "" {
		return nil, errors.New("serverName cannot be empty")
	}

	// Sanitize serverName to match storage key format
	serverName = strings.TrimSpace(serverName)
	serverName = strings.Map(func(r rune) rune {
		switch r {
		case '/', '\\', ' ', ':', '.', '*':
			return '_'
		default:
			return r
		}
	}, serverName)

	key := fmt.Sprintf("aws/sts/%s", serverName)
	opts := secret.SetOpts{Provider: secret.Credstore}

	sec, err := secret.Get(ctx, key, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get secret %s: %w", key, err)
	}
	if sec == nil || sec.Val == "" {
		return nil, fmt.Errorf("no credentials found for %s", serverName)
	}

	var creds AWSCredentials
	if err := json.Unmarshal([]byte(sec.Val), &creds); err != nil {
		return nil, fmt.Errorf("invalid credentials format for %s: %w", serverName, err)
	}

	if creds.AccessKeyId == "" || creds.Expiration.IsZero() {
		return nil, fmt.Errorf("incomplete credentials for %s", serverName)
	}

	if creds.Expiration.Before(time.Now()) {
		return nil, fmt.Errorf("credentials for %s are expired (since %s)",
			serverName, creds.Expiration.Format(time.RFC3339))
	}

	return &creds, nil
}

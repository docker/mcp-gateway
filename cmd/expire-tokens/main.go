package main

// Utility to expire OAuth tokens for testing token refresh flows
// Usage: go run ./cmd/expire-tokens <server-name> [seconds]
// Example: go run ./cmd/expire-tokens notion-remote 30

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/docker/docker-credential-helpers/client"
	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/docker/mcp-gateway/pkg/desktop"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <server-name> [seconds-until-expiry]\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Example: %s notion-remote 30\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nExpires the OAuth token for <server-name> in <seconds-until-expiry> (default: 30 seconds)\n")
		os.Exit(1)
	}

	serverName := os.Args[1]

	// Parse expiry seconds (default 30)
	expirySeconds := 30
	if len(os.Args) >= 3 {
		if seconds, err := strconv.Atoi(os.Args[2]); err == nil && seconds > 0 {
			expirySeconds = seconds
		} else {
			fmt.Fprintf(os.Stderr, "Error: Invalid seconds value: %s\n", os.Args[2])
			os.Exit(1)
		}
	}

	// Get DCR client to find credential key
	authClient := desktop.NewAuthClient()
	dcrClient, err := authClient.GetDCRClient(context.Background(), serverName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: No DCR client found for %s: %v\n", serverName, err)
		fmt.Fprintf(os.Stderr, "Make sure the server is registered. Run: docker mcp oauth list\n")
		os.Exit(1)
	}

	credentialKey := fmt.Sprintf("%s/%s", dcrClient.AuthorizationEndpoint, dcrClient.ProviderName)
	fmt.Fprintf(os.Stderr, "- Using credential key: %s\n", credentialKey)

	// Get existing token from credential helper
	program := client.NewShellProgramFunc("docker-credential-desktop")

	creds, err := client.Get(program, credentialKey)
	if err != nil {
		if credentials.IsErrCredentialsNotFound(err) {
			fmt.Fprintf(os.Stderr, "Error: No OAuth token found for %s\n", serverName)
			fmt.Fprintf(os.Stderr, "Please authorize first: docker mcp oauth authorize %s\n", serverName)
		} else {
			fmt.Fprintf(os.Stderr, "Error: Failed to get token for %s: %v\n", serverName, err)
		}
		os.Exit(1)
	}

	tokenSecret := creds.Secret

	// Decode and parse token JSON
	tokenJSON, err := base64.StdEncoding.DecodeString(tokenSecret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to decode token: %v\n", err)
		os.Exit(1)
	}

	var tokenData map[string]interface{}
	if err := json.Unmarshal(tokenJSON, &tokenData); err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to parse token JSON: %v\n", err)
		os.Exit(1)
	}

	// Display current expiry if available
	if expiryStr, ok := tokenData["expiry"].(string); ok {
		if currentExpiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
			fmt.Fprintf(os.Stderr, "- Current token expires at: %s\n", currentExpiry.Format(time.RFC3339))
		}
	}

	// Set token to expire in specified seconds
	// oauth2.Token uses "expiry" field with RFC3339 format, not "expires_at" with unix timestamp
	expiryTime := time.Now().Add(time.Duration(expirySeconds) * time.Second)
	tokenData["expiry"] = expiryTime.Format(time.RFC3339)
	delete(tokenData, "expires_at")  // Remove if exists (wrong field name)
	delete(tokenData, "expires_in")  // Remove relative expiry to avoid confusion

	fmt.Fprintf(os.Stderr, "\n📝 Preparing to write token:\n")
	fmt.Fprintf(os.Stderr, "   Key: %s\n", credentialKey)
	fmt.Fprintf(os.Stderr, "   Username: %s\n", dcrClient.ProviderName)
	fmt.Fprintf(os.Stderr, "   Expiry: %s\n", expiryTime.Format(time.RFC3339))

	// Re-encode and store
	newTokenJSON, err := json.Marshal(tokenData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to encode token: %v\n", err)
		os.Exit(1)
	}
	newTokenSecret := base64.StdEncoding.EncodeToString(newTokenJSON)

	// Delete credential first to clear Docker Desktop's in-memory cache
	// DD caches credentials and won't see external changes without this
	fmt.Fprintf(os.Stderr, "\n🗑️  Deleting credential to clear DD cache...\n")
	if err := client.Erase(program, credentialKey); err != nil {
		fmt.Fprintf(os.Stderr, "   (Delete returned: %v - continuing)\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "   ✓ Credential deleted\n")
	}

	fmt.Fprintf(os.Stderr, "\n📤 Storing updated token...\n")
	err = client.Store(program, &credentials.Credentials{
		ServerURL: credentialKey,
		Username:  dcrClient.ProviderName,
		Secret:    newTokenSecret,
	})

	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Store failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "✓ Store command completed without error\n")

	// Verify the token was stored correctly by reading it back
	fmt.Fprintf(os.Stderr, "\n📥 Reading token back to verify...\n")
	verifyCreds, err := client.Get(program, credentialKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed - couldn't read token: %v\n", err)
		fmt.Fprintf(os.Stderr, "   This means the Store() didn't actually save the credential!\n")
		os.Exit(1)
	}

	verifyJSON, err := base64.StdEncoding.DecodeString(verifyCreds.Secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed - couldn't decode: %v\n", err)
		os.Exit(1)
	}

	var verifyData map[string]interface{}
	if err := json.Unmarshal(verifyJSON, &verifyData); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed - couldn't parse JSON: %v\n", err)
		os.Exit(1)
	}

	expiryStr, ok := verifyData["expiry"].(string)
	if !ok {
		fmt.Fprintf(os.Stderr, "❌ Verification failed - no 'expiry' field in token!\n")
		fmt.Fprintf(os.Stderr, "   Token data: %+v\n", verifyData)
		os.Exit(1)
	}

	verifyExpiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Verification failed - couldn't parse expiry: %v\n", err)
		os.Exit(1)
	}

	// Check if the expiry matches what we wrote
	if verifyExpiry.Unix() != expiryTime.Unix() {
		fmt.Fprintf(os.Stderr, "❌ EXPIRY MISMATCH!\n")
		fmt.Fprintf(os.Stderr, "   Wrote:  %s (%d)\n", expiryTime.Format(time.RFC3339), expiryTime.Unix())
		fmt.Fprintf(os.Stderr, "   Read:   %s (%d)\n", verifyExpiry.Format(time.RFC3339), verifyExpiry.Unix())
		fmt.Fprintf(os.Stderr, "   The credential helper did NOT save our change!\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "✅ Verified: Token expiry correctly stored as %s\n", expiryStr)

	fmt.Fprintf(os.Stderr, "\n✓ Token for %s will expire in %d seconds\n", serverName, expirySeconds)
	fmt.Fprintf(os.Stderr, "  Expiry time: %s\n", expiryTime.Format(time.RFC3339))
	fmt.Fprintf(os.Stderr, "\nTo test proactive refresh:\n")
	fmt.Fprintf(os.Stderr, "  1. Wait %d seconds for token to approach expiry\n", expirySeconds-10)
	fmt.Fprintf(os.Stderr, "  2. Call a tool: docker mcp call %s <tool-name>\n", serverName)
	fmt.Fprintf(os.Stderr, "  3. Watch logs for '⚠ Token needs refresh' message\n\n")
}

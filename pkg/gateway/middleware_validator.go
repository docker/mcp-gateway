package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
)

// criticalMethods lists MCP methods that need duplicate key validation
// to prevent message smuggling attacks.
var criticalMethods = map[string]bool{
	"tools/call":     true,
	"prompts/get":    true,
	"resources/read": true,
}

// validateJSONMiddleware returns a middleware that validates JSON structure
// of incoming MCP messages to prevent message smuggling attacks.
//
// The middleware checks for duplicate keys with different cases (e.g., "name"
// and "Name") which can be used to bypass authorization by exploiting Go's
// case-insensitive JSON unmarshalling behavior.
//
// This provides defense-in-depth at the gateway level.
func (g *Gateway) validateJSONMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only validate critical methods that are susceptible to smuggling
			if !needsValidation(method) {
				return next(ctx, method, req)
			}

			// Get raw params and validate JSON structure
			params := req.GetParams()
			if params != nil {
				rawJSON, err := json.Marshal(params)
				if err != nil {
					log.Log(fmt.Sprintf("Warning: Failed to marshal params for validation: %v", err))
					// Continue with request - marshaling error will be caught elsewhere
					return next(ctx, method, req)
				}

				if err := validateJSONStructure(method, rawJSON); err != nil {
					log.Log(fmt.Sprintf("✗ JSON validation failed for %s: %v", method, err))
					return nil, fmt.Errorf("validation failed for %s: %w", method, err)
				}
			}

			return next(ctx, method, req)
		}
	}
}

// needsValidation returns true if the method requires JSON validation
func needsValidation(method string) bool {
	return criticalMethods[method]
}

// validateJSONStructure validates a JSON message for duplicate keys
func validateJSONStructure(_ string, rawJSON []byte) error {
	// Check for duplicate keys with different cases (the core vulnerability)
	if err := checkDuplicateKeys(rawJSON); err != nil {
		return fmt.Errorf("duplicate keys detected: %w", err)
	}

	return nil
}

// checkDuplicateKeys detects case-variant duplicates in JSON
func checkDuplicateKeys(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Not an object, no duplicates possible
		return nil
	}

	// Check for case-variant duplicates
	seen := make(map[string]string) // lowercase -> original
	for key := range raw {
		lowerKey := strings.ToLower(key)
		if original, exists := seen[lowerKey]; exists && original != key {
			return fmt.Errorf("found %q and %q (case variants)", original, key)
		}
		seen[lowerKey] = key
	}

	// Recursively check nested objects
	for key, val := range raw {
		if err := checkDuplicateKeysRecursive(val); err != nil {
			return fmt.Errorf("in field %q: %w", key, err)
		}
	}

	return nil
}

// checkDuplicateKeysRecursive recursively validates nested structures
func checkDuplicateKeysRecursive(data json.RawMessage) error {
	// Try as object
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(data, &obj); err == nil {
		seen := make(map[string]string)
		for key := range obj {
			lowerKey := strings.ToLower(key)
			if original, exists := seen[lowerKey]; exists && original != key {
				return fmt.Errorf("found %q and %q (case variants)", original, key)
			}
			seen[lowerKey] = key
		}

		// Recurse
		for key, val := range obj {
			if err := checkDuplicateKeysRecursive(val); err != nil {
				return fmt.Errorf("in field %q: %w", key, err)
			}
		}
		return nil
	}

	// Try as array
	var arr []json.RawMessage
	if err := json.Unmarshal(data, &arr); err == nil {
		for i, elem := range arr {
			if err := checkDuplicateKeysRecursive(elem); err != nil {
				return fmt.Errorf("in array index %d: %w", i, err)
			}
		}
		return nil
	}

	// Primitive value, no duplicates
	return nil
}

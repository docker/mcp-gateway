package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/log"
)

// MCPFieldSpecs defines the expected field names for critical MCP methods.
// This is used to validate that incoming messages conform to the MCP specification.
var MCPFieldSpecs = map[string]map[string]bool{
	"tools/call": {
		"name":      true,
		"arguments": true,
		"_meta":     true,
	},
	"prompts/get": {
		"name":      true,
		"arguments": true,
		"_meta":     true,
	},
	"resources/read": {
		"uri":   true,
		"_meta": true,
	},
}

// validateJSONMiddleware returns a middleware that validates JSON structure
// of incoming MCP messages to prevent message smuggling attacks.
//
// The middleware checks for:
// - Duplicate keys with different cases (e.g., "name" and "Name")
// - Field names that don't match expected MCP spec (case-sensitive)
// - Nested parameter validation
//
// This provides defense-in-depth at the gateway level, even if the SDK
// is not updated with strict parsing.
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
	_, exists := MCPFieldSpecs[method]
	return exists
}

// validateJSONStructure orchestrates all validation checks for a JSON message
func validateJSONStructure(method string, rawJSON []byte) error {
	// Check for duplicate keys with different cases
	if err := checkDuplicateKeys(rawJSON); err != nil {
		return fmt.Errorf("duplicate keys detected: %w", err)
	}

	// Check field names match expected spec
	if err := checkFieldNames(method, rawJSON); err != nil {
		return fmt.Errorf("invalid field names: %w", err)
	}

	// Check nested params (arguments field)
	if err := checkNestedParams(rawJSON); err != nil {
		return fmt.Errorf("invalid nested params: %w", err)
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

// checkFieldNames validates that JSON field names match the MCP spec exactly
func checkFieldNames(method string, data []byte) error {
	expectedFields, ok := MCPFieldSpecs[method]
	if !ok {
		// No spec for this method
		return nil
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// Not an object
		return nil
	}

	// Check each field name
	for key := range raw {
		// Check if this key exists in expected fields (case-sensitive)
		if !expectedFields[key] {
			// Check if a case-insensitive match exists (smuggling attempt)
			lowerKey := strings.ToLower(key)
			for expected := range expectedFields {
				if strings.ToLower(expected) == lowerKey {
					return fmt.Errorf("field %q has wrong case, expected %q", key, expected)
				}
			}
			// Unknown field - this is okay, the spec allows additional fields
			// Just log it for visibility
			log.Log(fmt.Sprintf("Warning: Unexpected field %q in %s", key, method))
		}
	}

	return nil
}

// checkNestedParams validates nested arguments/params objects
func checkNestedParams(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}

	// Check the "arguments" field if it exists
	if argsRaw, ok := raw["arguments"]; ok {
		if err := checkDuplicateKeysRecursive(argsRaw); err != nil {
			return fmt.Errorf("in arguments: %w", err)
		}
	}

	return nil
}

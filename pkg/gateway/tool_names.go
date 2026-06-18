package gateway

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

var errToolNameCollision = errors.New("tool name collision")

type toolNameCollisionError struct {
	message string
}

func (e toolNameCollisionError) Error() string {
	return e.message
}

func (e toolNameCollisionError) Is(target error) bool {
	return target == errToolNameCollision
}

var reservedGatewayToolNames = map[string]struct{}{
	"code-mode":            {},
	"find-tools":           {},
	"mcp-activate-profile": {},
	"mcp-add":              {},
	"mcp-config-set":       {},
	"mcp-create-profile":   {},
	"mcp-exec":             {},
	"mcp-find":             {},
	"mcp-registry-import":  {},
	"mcp-remove":           {},
}

func validateExternalToolNameCollisions(registrations []ToolRegistration, existing map[string]ToolRegistration) error {
	return validateToolNameCollisions(registrations, existing, true)
}

func validateToolNameCollisions(registrations []ToolRegistration, existing map[string]ToolRegistration, rejectReserved bool) error {
	seen := make(map[string]ToolRegistration, len(registrations))

	for _, registration := range sortedToolRegistrations(registrations) {
		if registration.Tool == nil {
			continue
		}

		toolName := strings.TrimSpace(registration.Tool.Name)
		if toolName == "" {
			return toolNameCollisionError{message: fmt.Sprintf("tool name collision: %s exposes an empty tool name", toolOwner(registration))}
		}

		if rejectReserved {
			if _, reserved := reservedGatewayToolNames[toolName]; reserved {
				return toolNameCollisionError{message: fmt.Sprintf("tool name collision: %s exposes reserved gateway tool name %q; enable tool-name-prefix or set a unique catalog prefix", toolOwner(registration), toolName)}
			}
		}

		if previous, ok := seen[toolName]; ok {
			return toolNameCollisionError{message: fmt.Sprintf("tool name collision: %s and %s both expose tool name %q; enable tool-name-prefix or set unique catalog prefixes", toolOwner(previous), toolOwner(registration), toolName)}
		}
		seen[toolName] = registration

		if existing == nil {
			continue
		}
		if previous, ok := existing[toolName]; ok {
			return toolNameCollisionError{message: fmt.Sprintf("tool name collision: %s would shadow %s for tool name %q; enable tool-name-prefix or set a unique catalog prefix", toolOwner(registration), toolOwner(previous), toolName)}
		}
	}

	return nil
}

func sortedToolRegistrations(registrations []ToolRegistration) []ToolRegistration {
	sorted := append([]ToolRegistration(nil), registrations...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.ServerName != right.ServerName {
			return left.ServerName < right.ServerName
		}
		var leftName, rightName string
		if left.Tool != nil {
			leftName = left.Tool.Name
		}
		if right.Tool != nil {
			rightName = right.Tool.Name
		}
		return leftName < rightName
	})
	return sorted
}

func toolOwner(registration ToolRegistration) string {
	if registration.ServerName == "" {
		return "gateway internal tools"
	}
	return fmt.Sprintf("server %q", registration.ServerName)
}

func (g *Gateway) addCatalogToolNameDiagnostics(serverInfo map[string]any, serverName string, server catalog.Server) {
	warnings := g.catalogToolNameWarnings(serverName, server)
	if len(warnings) > 0 {
		serverInfo["tool_name_warnings"] = warnings
	}
}

func (g *Gateway) catalogToolNameWarnings(serverName string, server catalog.Server) []string {
	if len(server.Tools) == 0 {
		return nil
	}

	prefix := server.Prefix
	if prefix == "" && g.ToolNamePrefix {
		prefix = serverName
	}

	g.capabilitiesMu.RLock()
	existing := make(map[string]ToolRegistration, len(g.toolRegistrations))
	for name, registration := range g.toolRegistrations {
		existing[name] = registration
	}
	g.capabilitiesMu.RUnlock()

	var warnings []string
	seen := make(map[string]string, len(server.Tools))
	for _, tool := range server.Tools {
		toolName := strings.TrimSpace(tool.Name)
		if toolName == "" {
			warnings = append(warnings, "catalog metadata includes an empty tool name")
			continue
		}

		exposedName := prefixToolName(prefix, toolName)
		if previousRawName, ok := seen[exposedName]; ok {
			warnings = append(warnings, fmt.Sprintf("tool %q would be exposed as %q, which duplicates tool %q in this server", toolName, exposedName, previousRawName))
			continue
		}
		seen[exposedName] = toolName

		if _, reserved := reservedGatewayToolNames[exposedName]; reserved {
			warnings = append(warnings, fmt.Sprintf("tool %q would be exposed as %q, which is reserved for a gateway internal tool", toolName, exposedName))
			continue
		}

		if previous, ok := existing[exposedName]; ok && previous.ServerName != serverName {
			warnings = append(warnings, fmt.Sprintf("tool %q would be exposed as %q, which conflicts with %s", toolName, exposedName, toolOwner(previous)))
		}
	}

	sort.Strings(warnings)
	return warnings
}

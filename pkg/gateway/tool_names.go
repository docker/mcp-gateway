package gateway

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"unicode"

	"github.com/docker/mcp-gateway/pkg/catalog"
)

const maxMcpToolNameLength = 64

var validMcpToolNameRune = map[rune]bool{
	'_': true,
	'-': true,
}

var (
	errToolNameCollision       = errors.New("tool name collision")
	errCapabilityNameCollision = errors.New("capability name collision")
)

type toolNameCollisionError struct {
	message string
}

func (e toolNameCollisionError) Error() string {
	return e.message
}

func (e toolNameCollisionError) Is(target error) bool {
	return target == errToolNameCollision
}

type capabilityNameCollisionError struct {
	message string
}

func (e capabilityNameCollisionError) Error() string {
	return e.message
}

func (e capabilityNameCollisionError) Is(target error) bool {
	return target == errCapabilityNameCollision
}

func isCapabilityNameCollision(err error) bool {
	return errors.Is(err, errToolNameCollision) || errors.Is(err, errCapabilityNameCollision)
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

var reservedGatewayPromptNames = map[string]struct{}{
	"mcp-discover": {},
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

// sanitizeMcpToolName normalizes upstream tool names to the MCP pattern
// accepted by strict clients (^[a-zA-Z0-9_-]{1,64}$).
func sanitizeMcpToolName(name string) string {
	var b strings.Builder
	b.Grow(len(name))

	prevUnderscore := false
	for _, r := range name {
		valid := unicode.IsLetter(r) || unicode.IsDigit(r) || validMcpToolNameRune[r]
		if valid {
			b.WriteRune(r)
			prevUnderscore = false
			continue
		}
		if !prevUnderscore {
			b.WriteRune('_')
			prevUnderscore = true
		}
	}

	s := strings.Trim(b.String(), "_")
	if s == "" {
		return "tool"
	}
	if len(s) > maxMcpToolNameLength {
		s = strings.TrimRight(s[:maxMcpToolNameLength], "_")
		if s == "" {
			return "tool"
		}
	}
	return s
}

// exposeToolName applies the configured prefix and sanitizes the exposed name.
func exposeToolName(prefix, toolName string) string {
	return sanitizeMcpToolName(prefixToolName(prefix, toolName))
}

// uniqueExposeToolName returns a sanitized exposed name that is unique within seen.
func uniqueExposeToolName(prefix, toolName string, seen map[string]struct{}) string {
	base := exposeToolName(prefix, toolName)
	name := base
	for i := 2; ; i++ {
		if _, ok := seen[name]; !ok {
			seen[name] = struct{}{}
			return name
		}

		suffix := fmt.Sprintf("_%d", i)
		maxBase := maxMcpToolNameLength - len(suffix)
		trimmed := base
		if len(trimmed) > maxBase {
			trimmed = strings.TrimRight(trimmed[:maxBase], "_")
		}
		if trimmed == "" {
			name = fmt.Sprintf("tool%s", suffix)
			continue
		}
		name = trimmed + suffix
	}
}

func toolOwner(registration ToolRegistration) string {
	if registration.ServerName == "" {
		return "gateway internal tools"
	}
	return fmt.Sprintf("server %q", registration.ServerName)
}

type capabilityNameIndexes struct {
	Prompts           map[string]string
	Resources         map[string]string
	ResourceTemplates map[string]string
}

type capabilityIdentityRegistration struct {
	serverName string
	identifier string
}

func validateExternalCapabilityNameCollisions(caps *Capabilities, existing capabilityNameIndexes, rejectReservedPrompts bool) error {
	if caps == nil {
		return nil
	}

	var reservedPrompts map[string]struct{}
	if rejectReservedPrompts {
		reservedPrompts = reservedGatewayPromptNames
	}
	if err := validateCapabilityIdentityCollisions(
		"prompt name",
		promptIdentities(caps.Prompts),
		existing.Prompts,
		reservedPrompts,
		"disable one server or expose unique prompt names",
	); err != nil {
		return err
	}
	if err := validateCapabilityIdentityCollisions(
		"resource URI",
		resourceIdentities(caps.Resources),
		existing.Resources,
		nil,
		"disable one server or expose unique resource URIs",
	); err != nil {
		return err
	}
	if err := validateCapabilityIdentityCollisions(
		"resource template URI template",
		resourceTemplateIdentities(caps.ResourceTemplates),
		existing.ResourceTemplates,
		nil,
		"disable one server or expose unique resource template URI templates",
	); err != nil {
		return err
	}

	return nil
}

func validateCapabilityIdentityCollisions(kind string, registrations []capabilityIdentityRegistration, existing map[string]string, reserved map[string]struct{}, mitigation string) error {
	seen := make(map[string]capabilityIdentityRegistration, len(registrations))

	for _, registration := range sortedCapabilityIdentityRegistrations(registrations) {
		if strings.TrimSpace(registration.identifier) == "" {
			return capabilityNameCollisionError{message: fmt.Sprintf("%s collision: %s exposes an empty %s", kind, capabilityOwner(registration.serverName), kind)}
		}

		if _, ok := reserved[registration.identifier]; ok {
			return capabilityNameCollisionError{message: fmt.Sprintf("%s collision: %s exposes reserved gateway %s %q; %s", kind, capabilityOwner(registration.serverName), kind, registration.identifier, mitigation)}
		}

		if previous, ok := seen[registration.identifier]; ok {
			return capabilityNameCollisionError{message: fmt.Sprintf("%s collision: %s and %s both expose %s %q; %s", kind, capabilityOwner(previous.serverName), capabilityOwner(registration.serverName), kind, registration.identifier, mitigation)}
		}
		seen[registration.identifier] = registration

		if existing == nil {
			continue
		}
		if previousServerName, ok := existing[registration.identifier]; ok {
			return capabilityNameCollisionError{message: fmt.Sprintf("%s collision: %s would shadow %s for %s %q; %s", kind, capabilityOwner(registration.serverName), capabilityOwner(previousServerName), kind, registration.identifier, mitigation)}
		}
	}

	return nil
}

func promptIdentities(registrations []PromptRegistration) []capabilityIdentityRegistration {
	identities := make([]capabilityIdentityRegistration, 0, len(registrations))
	for _, registration := range registrations {
		if registration.Prompt == nil {
			continue
		}
		identities = append(identities, capabilityIdentityRegistration{
			serverName: registration.ServerName,
			identifier: registration.Prompt.Name,
		})
	}
	return identities
}

func resourceIdentities(registrations []ResourceRegistration) []capabilityIdentityRegistration {
	identities := make([]capabilityIdentityRegistration, 0, len(registrations))
	for _, registration := range registrations {
		if registration.Resource == nil {
			continue
		}
		identities = append(identities, capabilityIdentityRegistration{
			serverName: registration.ServerName,
			identifier: registration.Resource.URI,
		})
	}
	return identities
}

func resourceTemplateIdentities(registrations []ResourceTemplateRegistration) []capabilityIdentityRegistration {
	identities := make([]capabilityIdentityRegistration, 0, len(registrations))
	for _, registration := range registrations {
		identities = append(identities, capabilityIdentityRegistration{
			serverName: registration.ServerName,
			identifier: registration.ResourceTemplate.URITemplate,
		})
	}
	return identities
}

func sortedCapabilityIdentityRegistrations(registrations []capabilityIdentityRegistration) []capabilityIdentityRegistration {
	sorted := append([]capabilityIdentityRegistration(nil), registrations...)
	sort.SliceStable(sorted, func(i, j int) bool {
		left, right := sorted[i], sorted[j]
		if left.serverName != right.serverName {
			return left.serverName < right.serverName
		}
		return left.identifier < right.identifier
	})
	return sorted
}

func capabilityOwner(serverName string) string {
	if serverName == "" {
		return "gateway internal capabilities"
	}
	return fmt.Sprintf("server %q", serverName)
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

		exposedName := exposeToolName(prefix, toolName)
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

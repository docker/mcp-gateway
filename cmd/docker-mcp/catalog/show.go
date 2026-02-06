package catalog

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/docker/cli/cli/command"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/hints"
	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/policy"
	policycli "github.com/docker/mcp-gateway/pkg/policy/cli"
	policycontext "github.com/docker/mcp-gateway/pkg/policy/context"
	"github.com/docker/mcp-gateway/pkg/terminal"
)

type Format string

const (
	JSON Format = "json"
	YAML Format = "yaml"
)

var supportedFormats = []Format{JSON, YAML}

// catalogDocument models the catalog YAML structure for output.
type catalogDocument struct {
	// Name is the catalog identifier.
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
	// DisplayName is the catalog display name.
	DisplayName string `yaml:"displayName,omitempty" json:"displayName,omitempty"`
	// Registry holds the catalog servers.
	Registry map[string]catalog.Server `yaml:"registry" json:"registry"`
	// Policy describes the catalog policy decision.
	Policy *policy.Decision `yaml:"policy,omitempty" json:"policy,omitempty"`
}

func (e *Format) String() string {
	return string(*e)
}

func (e *Format) Set(v string) error {
	actual := Format(v)
	for _, allowed := range supportedFormats {
		if allowed == actual {
			*e = actual
			return nil
		}
	}
	return fmt.Errorf("must be one of %s", SupportedFormats())
}

// Type is only used in help text
func (e *Format) Type() string {
	return "format"
}

func SupportedFormats() string {
	var quoted []string
	for _, v := range supportedFormats {
		quoted = append(quoted, "\""+string(v)+"\"")
	}
	return strings.Join(quoted, ", ")
}

// Show displays catalog contents with policy information.
func Show(ctx context.Context, dockerCli command.Cli, name string, format Format, mcpOAuthDcrEnabled bool) error {
	cfg, err := ReadConfigWithDefaultCatalog(ctx)
	if err != nil {
		return err
	}
	catalog, ok := cfg.Catalogs[name]
	if !ok {
		return fmt.Errorf("catalog %q not found", name)
	}

	// Auto update the catalog if it's "too old"
	needsUpdate := false
	if name == DockerCatalogName && isURL(catalog.URL) {
		if catalog.LastUpdate == "" {
			needsUpdate = true
		} else {
			lastUpdated, err := time.Parse(time.RFC3339, catalog.LastUpdate)
			if err != nil {
				needsUpdate = true
			} else if lastUpdated.Add(12 * time.Hour).Before(time.Now()) {
				needsUpdate = true
			}
		}
	}
	if !needsUpdate {
		_, err := ReadCatalogFile(name)
		if errors.Is(err, os.ErrNotExist) {
			needsUpdate = true
		}
	}
	if needsUpdate {
		if err := updateCatalog(ctx, name, catalog, mcpOAuthDcrEnabled); err != nil {
			return err
		}
	}

	data, err := ReadCatalogFile(name)
	if err != nil {
		return err
	}

	var document catalogDocument
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("failed to unmarshal catalog data: %w", err)
	}
	catalogID := catalogID(name, document.Name)
	ctxData := policycontext.Context{
		Catalog:                  catalogID,
		WorkingSet:               "",
		ServerSourceTypeOverride: "registry",
	}
	policyClient := policycli.ClientForCLI(ctx)
	showPolicy := policyClient != nil
	document.Policy = policycli.DecisionForRequest(
		ctx,
		policyClient,
		policycontext.BuildCatalogRequest(ctxData, catalogID, policy.ActionLoad),
	)
	applyCatalogPolicy(ctx, policyClient, ctxData, document.Registry)

	if format != "" {
		return writeCatalogOutput(format, document)
	}

	keys := getSortedKeys(document.Registry)

	termWidth := terminal.GetWidth()
	wrapWidth := termWidth - 10
	if wrapWidth < 40 {
		wrapWidth = 40
	}

	serverCount := len(keys)
	headerLineWidth := termWidth - 4
	if headerLineWidth > 78 {
		headerLineWidth = 78
	}

	fmt.Println()
	fmt.Printf("  \033[1mMCP Server Directory\033[0m\n")
	fmt.Printf("  %d servers available\n", serverCount)
	if showPolicy {
		fmt.Printf("  Policy: %s\n", policycli.StatusMessage(document.Policy))
	}
	fmt.Printf("  %s\n", strings.Repeat("─", headerLineWidth))
	fmt.Println()

	for i, k := range keys {
		val, ok := document.Registry[k]
		if !ok {
			continue
		}
		fmt.Printf("  \033[1m%s\033[0m\n", k)
		wrappedDesc := wrapText(strings.TrimSpace(val.Description), wrapWidth, "    ")
		fmt.Println(wrappedDesc)
		if showPolicy {
			fmt.Printf("    Policy: %s\n", policycli.StatusMessage(val.Policy))
		}

		if i < len(keys)-1 {
			fmt.Println()
		}
	}

	fmt.Println()
	fmt.Printf("  %s\n", strings.Repeat("─", headerLineWidth))
	fmt.Printf("  %d servers total\n", serverCount)
	fmt.Println()
	if hints.Enabled(dockerCli) {
		hints.TipCyan.Print("Tip: To view server details, use ")
		hints.TipCyanBoldItalic.Print("docker mcp server inspect <server-name>")
		hints.TipCyan.Print(". To add servers, use ")
		hints.TipCyanBoldItalic.Println("docker mcp server enable <server-name>")
	}

	return nil
}

// getSortedKeys returns catalog keys in sorted order.
func getSortedKeys(m map[string]catalog.Server) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// catalogID returns the catalog identifier for policy context.
func catalogID(fallback, configured string) string {
	if configured != "" {
		return configured
	}
	return fallback
}

// applyCatalogPolicy attaches policy decisions to catalog servers and tools.
// It uses batch evaluation to minimize HTTP overhead when evaluating many
// servers and tools.
func applyCatalogPolicy(
	ctx context.Context,
	client policy.Client,
	ctxData policycontext.Context,
	servers map[string]catalog.Server,
) {
	if client == nil {
		return
	}

	// Metadata for mapping batch results back to servers/tools.
	type serverMeta struct {
		name  string
		index int // Index in batch request.
	}
	type toolMeta struct {
		serverName  string
		toolIndex   int
		loadIndex   int // Index in batch request for load action.
		invokeIndex int // Index in batch request for invoke action.
	}

	var requests []policy.Request
	var serverMetas []serverMeta
	var toolMetas []toolMeta

	// Build batch request with all policy evaluations.
	for name, server := range servers {
		// Add server load policy request.
		serverMetas = append(serverMetas, serverMeta{name: name, index: len(requests)})
		requests = append(requests, policycontext.BuildRequest(
			ctxData, name, server, "", policy.ActionLoad,
		))

		// Add tool policy requests (both load and invoke).
		for i, tool := range server.Tools {
			loadIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				ctxData, name, server, tool.Name, policy.ActionLoad,
			))
			invokeIndex := len(requests)
			requests = append(requests, policycontext.BuildRequest(
				ctxData, name, server, tool.Name, policy.ActionInvoke,
			))
			toolMetas = append(toolMetas, toolMeta{
				serverName:  name,
				toolIndex:   i,
				loadIndex:   loadIndex,
				invokeIndex: invokeIndex,
			})
		}
	}

	if len(requests) == 0 {
		return
	}

	// Evaluate all requests in a single batch call.
	decisions, err := client.EvaluateBatch(ctx, requests)
	decisions, _ = policycli.NormalizeBatchDecisions(requests, decisions, err)

	// Apply server decisions.
	for _, sm := range serverMetas {
		server := servers[sm.name]
		server.Policy = policy.DecisionForOutput(decisions[sm.index])
		servers[sm.name] = server
	}

	// Apply tool decisions. Use load result if blocked, otherwise invoke.
	for _, tm := range toolMetas {
		server := servers[tm.serverName]
		loadDecision := policy.DecisionForOutput(decisions[tm.loadIndex])
		if loadDecision != nil {
			server.Tools[tm.toolIndex].Policy = loadDecision
		} else {
			server.Tools[tm.toolIndex].Policy = policy.DecisionForOutput(decisions[tm.invokeIndex])
		}
		servers[tm.serverName] = server
	}
}

// writeCatalogOutput prints catalog output for the selected format.
func writeCatalogOutput(format Format, document catalogDocument) error {
	var (
		data []byte
		err  error
	)
	switch format {
	case JSON:
		data, err = json.MarshalIndent(document, "", "  ")
	case YAML:
		data, err = yaml.Marshal(document)
	default:
		return fmt.Errorf("unsupported format %q", format)
	}
	if err != nil {
		return fmt.Errorf("transforming catalog data: %w", err)
	}
	fmt.Println(string(data))
	return nil
}

func isURL(fileOrURL string) bool {
	return strings.HasPrefix(fileOrURL, "http://") || strings.HasPrefix(fileOrURL, "https://")
}

func wrapText(text string, width int, indent string) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}

	var lines []string
	currentLine := words[0]

	for _, word := range words[1:] {
		if len(currentLine)+1+len(word) > width {
			lines = append(lines, indent+currentLine)
			currentLine = word
		} else {
			currentLine += " " + word
		}
	}
	lines = append(lines, indent+currentLine)

	return strings.Join(lines, "\n")
}

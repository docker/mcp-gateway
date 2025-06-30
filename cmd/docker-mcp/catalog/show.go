package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mikefarah/yq/v4/pkg/yqlib"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/yq"
)

type Format string

const (
	JSON Format = "json"
	YAML Format = "yaml"
)

var supportedFormats = []Format{JSON, YAML}

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
	return fmt.Errorf("must be one of %s", PrintSupportedFormats())
}

// Type is only used in help text
func (e *Format) Type() string {
	return "format"
}

func PrintSupportedFormats() string {
	var quoted []string
	for _, v := range supportedFormats {
		quoted = append(quoted, "\""+string(v)+"\"")
	}
	return strings.Join(quoted, ", ")
}

type showOpts struct {
	Format
}

func newShowCommand() *cobra.Command {
	opts := &showOpts{}
	cmd := &cobra.Command{
		Use:   "show <name>",
		Short: "Show a catalog",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd.Context(), args[0], *opts)
		},
	}
	flags := cmd.Flags()
	flags.Var(&opts.Format, "format", fmt.Sprintf("Supported: %s.", PrintSupportedFormats()))
	return cmd
}

func runShow(ctx context.Context, name string, opts showOpts) error {
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
	if name == DockerCatalogName {
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
		if err := updateCatalog(ctx, name, catalog); err != nil {
			return err
		}
	}

	data, err := ReadCatalogFile(name)
	if err != nil {
		return err
	}

	if opts.Format != "" {
		var encoder yqlib.Encoder
		switch opts.Format {
		case JSON:
			encoder = yq.NewJSONEncoder()
		case YAML:
			encoder = yq.NewYamlEncoder()
		default:
			return fmt.Errorf("unsupported format %q", opts.Format)
		}
		transformed, err := yq.Evaluate(".", data, yq.NewYamlDecoder(), encoder)
		if err != nil {
			return fmt.Errorf("transforming catalog data: %w", err)
		}
		fmt.Println(string(transformed))
		return nil
	}
	var registry Registry
	if err := yaml.Unmarshal(data, &registry); err != nil {
		return fmt.Errorf("failed to unmarshal catalog data: %w", err)
	}
	keys := getSortedKeys(registry.Registry)
	for _, k := range keys {
		val, ok := registry.Registry[k]
		if !ok {
			continue
		}
		fmt.Printf("%s: %s\n", k, strings.TrimSpace(val.Description))
	}
	return nil
}

func getSortedKeys(m map[string]Tile) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

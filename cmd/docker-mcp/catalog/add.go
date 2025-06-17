package catalog

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/yq"
)

type addOpts struct {
	Force bool
}

func newAddCommand() *cobra.Command {
	opts := &addOpts{}
	cmd := &cobra.Command{
		Use:   "add <catalog> <source-catalog>/<server-name>",
		Short: "Add a server to your catalog",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			parsedArgs, err := parseAddArgs(args[0], args[1])
			if err != nil {
				return err
			}
			if err := validateArgs(*parsedArgs); err != nil {
				return err
			}
			return runAdd(*parsedArgs, *opts)
		},
		Hidden: true,
	}
	flags := cmd.Flags()
	flags.BoolVar(&opts.Force, "force", false, "Overwrite existing server in the catalog")
	return cmd
}

type ParsedAddArgs struct {
	Src       string
	Dst       string
	SeverName string
}

func parseAddArgs(dst, src string) (*ParsedAddArgs, error) {
	srcParts := strings.Split(src, "/")
	if len(srcParts) != 2 {
		return nil, fmt.Errorf("cannot parse %s: expected format <source-catalog>/<server-name>", src)
	}
	return &ParsedAddArgs{
		Src:       srcParts[0],
		Dst:       dst,
		SeverName: srcParts[1],
	}, nil
}

func validateArgs(args ParsedAddArgs) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	if _, ok := cfg.Catalogs[args.Src]; !ok {
		return fmt.Errorf("catalog %q not found", args.Src)
	}
	dst, ok := cfg.Catalogs[args.Dst]
	if !ok {
		return fmt.Errorf("catalog %q not found", args.Dst)
	}
	if dst.URL != "" {
		return fmt.Errorf("catalog %q is a remote catalog", args.Dst)
	}
	if args.Dst == args.Src {
		return errors.New("cannot add server to the same catalog")
	}
	return nil
}

func runAdd(args ParsedAddArgs, opts addOpts) error {
	srcContent, err := ReadCatalogFile(args.Src)
	if err != nil {
		return err
	}
	serverJSON, err := extractServerJSON(srcContent, args.SeverName)
	if err != nil {
		return err
	}
	if len(serverJSON) == 0 {
		return fmt.Errorf("server %q not found in catalog %q", args.SeverName, args.Src)
	}
	dstContentBefore, err := ReadCatalogFile(args.Dst)
	if err != nil {
		return err
	}
	dstServerJSON, err := extractServerJSON(dstContentBefore, args.SeverName)
	if err == nil && len(dstServerJSON) > 0 && !opts.Force {
		fmt.Println(string(dstServerJSON))
		return fmt.Errorf("server %q already exists in catalog %q (use --force to overwrite)", args.SeverName, args.Dst)
	}
	dstContentAfter, err := injectServerJSON(dstContentBefore, args.SeverName, serverJSON)
	if err != nil {
		return err
	}
	if err := WriteCatalogFile(args.Dst, dstContentAfter); err != nil {
		return err
	}
	fmt.Printf("copied server \"%s\" from catalog \"%s\" to \"%s\"\n", args.SeverName, args.Src, args.Dst)
	return nil
}

func extractServerJSON(yamlData []byte, serverName string) ([]byte, error) {
	query := fmt.Sprintf(`.registry."%s"`, serverName)
	return yq.Evaluate(query, yamlData, yq.NewYamlDecoder(), yq.NewJSONEncoder())
}

func injectServerJSON(yamlData []byte, serverName string, serverJSON []byte) ([]byte, error) {
	query := fmt.Sprintf(`.registry."%s" = %s`, serverName, string(serverJSON))
	return yq.Evaluate(query, yamlData, yq.NewYamlDecoder(), yq.NewYamlEncoder())
}

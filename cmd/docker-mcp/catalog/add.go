package catalog

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/yq"
)

type addOpts struct {
	Force bool
}

func newAddCommand() *cobra.Command {
	opts := &addOpts{}
	cmd := &cobra.Command{
		Use:   "add <catalog> <server-name> <catalog-file>",
		Short: "Add a server to your catalog",
		Args:  cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			parsedArgs := parseAddArgs(args[0], args[1], args[2])
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

func parseAddArgs(dst, src, catalogFile string) *ParsedAddArgs {
	return &ParsedAddArgs{
		Src:       catalogFile,
		Dst:       dst,
		SeverName: src,
	}
}

func validateArgs(args ParsedAddArgs) error {
	cfg, err := ReadConfig()
	if err != nil {
		return err
	}
	// check if the catalog file exists
	if _, err := os.Stat(args.Src); os.IsNotExist(err) {
		return fmt.Errorf("catalog file %q not found", args.Src)
	}

	_, ok := cfg.Catalogs[args.Dst]
	if !ok {
		return fmt.Errorf("catalog %q not found", args.Dst)
	}

	if _, err := os.Stat(args.Src); os.IsNotExist(err) {
		return fmt.Errorf("source catalog %q not found", args.Dst)
	}

	if args.Dst == args.Src {
		return errors.New("cannot add server to the same catalog")
	}
	return nil
}

func runAdd(args ParsedAddArgs, opts addOpts) error {
	srcContent, err := os.ReadFile(args.Src)
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
	if err == nil && len(dstServerJSON) > 4 && !opts.Force {
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

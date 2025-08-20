package catalog

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/telemetry"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/yq"
)

type ParsedAddArgs struct {
	Src       string
	Dst       string
	SeverName string
}

func ParseAddArgs(dst, src, catalogFile string) *ParsedAddArgs {
	return &ParsedAddArgs{
		Src:       catalogFile,
		Dst:       dst,
		SeverName: src,
	}
}

func ValidateArgs(args ParsedAddArgs) error {
	// Prevent users from modifying the Docker catalog
	if args.Dst == DockerCatalogName {
		return fmt.Errorf("cannot add servers to catalog '%s' as it is managed by Docker", args.Dst)
	}

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

func Add(args ParsedAddArgs, force bool) error {
	// Initialize telemetry
	telemetry.Init()
	ctx := context.Background()

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		success := recover() == nil
		telemetry.RecordCatalogOperation(ctx, "add", args.Dst, float64(duration.Milliseconds()), success)
	}()

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
	if err == nil && len(dstServerJSON) > 4 && !force {
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

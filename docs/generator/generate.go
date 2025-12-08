package main

import (
	"context"
	"log"
	"os"
	"strings"

	clidocstool "github.com/docker/cli-docs-tool"
	"github.com/docker/cli/cli/command"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/commands"
	"github.com/docker/mcp-gateway/pkg/features"
)

const defaultSourcePath = "/reference/"

type options struct {
	source  string
	formats []string
}

func gen(opts *options) error {
	log.SetFlags(0)

	dockerCLI, err := command.NewDockerCli()
	if err != nil {
		return err
	}
	cmd := &cobra.Command{
		Use:               "docker [OPTIONS] COMMAND [ARG...]",
		Short:             "The base command for the Docker CLI.",
		DisableAutoGenTag: true,
	}

	cmd.AddCommand(commands.Root(context.TODO(), "", dockerCLI, features.AllDisabled()))

	c, err := clidocstool.New(clidocstool.Options{
		Root:      cmd,
		SourceDir: opts.source,
		Plugin:    true,
	})
	if err != nil {
		return err
	}

	for _, format := range opts.formats {
		switch format {
		case "md":
			if err = c.GenMarkdownTree(cmd); err != nil {
				return err
			}
		case "yaml":
			fixUpExperimentalCLI(cmd)
			if err = c.GenYamlTree(cmd); err != nil {
				return err
			}
		default:
			return errors.Errorf("unknown format %q", format)
		}
	}

	return nil
}

func run() error {
	opts := &options{}
	flags := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)
	flags.StringVar(&opts.source, "source", defaultSourcePath, "Docs source folder")
	flags.StringSliceVar(&opts.formats, "formats", []string{}, "Format (md, yaml)")
	if err := flags.Parse(os.Args[1:]); err != nil {
		return err
	}
	if len(opts.formats) == 0 {
		return errors.New("Docs format required")
	}
	return gen(opts)
}

func main() {
	if err := run(); err != nil {
		log.Printf("ERROR: %+v", err)
		os.Exit(1)
	}
}

// fixUpExperimentalCLI trims the " (EXPERIMENTAL)" suffix from the CLI output,
// as docs.docker.com already displays "experimental (CLI)",
//
// https://github.com/docker/buildx/pull/2188#issuecomment-1889487022
func fixUpExperimentalCLI(cmd *cobra.Command) {
	const (
		annotationExperimentalCLI = "experimentalCLI"
		suffixExperimental        = " (EXPERIMENTAL)"
	)
	if _, ok := cmd.Annotations[annotationExperimentalCLI]; ok {
		cmd.Short = strings.TrimSuffix(cmd.Short, suffixExperimental)
	}
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if _, ok := f.Annotations[annotationExperimentalCLI]; ok {
			f.Usage = strings.TrimSuffix(f.Usage, suffixExperimental)
		}
	})
	for _, c := range cmd.Commands() {
		fixUpExperimentalCLI(c)
	}
}

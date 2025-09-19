package gateway

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/eval"
	"github.com/docker/mcp-gateway/pkg/gateway/proxies"
)

func baseArgs(name string, opts Options) []string {
	args := []string{
		"run", "--rm", "-i", "--init",
		"--security-opt", "no-new-privileges",
		"--pull", "never",
		"-l", "docker-mcp=true",
		"-l", "docker-mcp-tool-type=mcp",
		"-l", "docker-mcp-name=" + name,
		"-l", "docker-mcp-transport=stdio",
	}

	if opts.Cpus > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", opts.Cpus))
	}
	if opts.Memory != "" {
		args = append(args, "--memory", opts.Memory)
	}
	if os.Getenv("DOCKER_MCP_IN_DIND") == "1" {
		args = append(args, "--privileged")
	}

	return args
}

func ArgsAndEnvForMCPServer(serverConfig *catalog.ServerConfig, readOnly *bool, targetConfig proxies.TargetConfig, opts Options, networks []string) ([]string, []string) {
	args := baseArgs(serverConfig.Name, opts)

	var env []string

	// Security options
	if serverConfig.Spec.DisableNetwork {
		args = append(args, "--network", "none")
	} else {
		// Attach the MCP servers to the same network as the gateway.
		for _, network := range networks {
			args = append(args, "--network", network)
		}
	}
	if targetConfig.NetworkName != "" {
		args = append(args, "--network", targetConfig.NetworkName)
	}
	for _, link := range targetConfig.Links {
		args = append(args, "--link", link)
	}
	for _, env := range targetConfig.Env {
		args = append(args, "-e", env)
	}
	if targetConfig.DNS != "" {
		args = append(args, "--dns", targetConfig.DNS)
	}

	// Secrets
	for _, s := range serverConfig.Spec.Secrets {
		args = append(args, "-e", s.Env)

		secretValue, ok := serverConfig.Secrets[s.Name]
		if ok {
			env = append(env, fmt.Sprintf("%s=%s", s.Env, secretValue))
		} else {
			logf("Warning: Secret '%s' not found for server '%s', setting %s=<UNKNOWN>", s.Name, serverConfig.Name, s.Env)
			env = append(env, fmt.Sprintf("%s=%s", s.Env, "<UNKNOWN>"))
		}
	}

	// Env
	for _, e := range serverConfig.Spec.Env {
		var value string
		if strings.Contains(e.Value, "{{") && strings.Contains(e.Value, "}}") {
			value = fmt.Sprintf("%v", eval.Evaluate(e.Value, serverConfig.Config))
		} else {
			value = expandEnv(e.Value, env)
		}

		if value != "" {
			args = append(args, "-e", e.Name)
			env = append(env, fmt.Sprintf("%s=%s", e.Name, value))
		}
	}

	// Volumes
	for _, mount := range eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config) {
		if mount == "" {
			continue
		}

		if readOnly != nil && *readOnly && !strings.HasSuffix(mount, ":ro") {
			args = append(args, "-v", mount+":ro")
		} else {
			args = append(args, "-v", mount)
		}
	}

	// User
	if serverConfig.Spec.User != "" {
		val := serverConfig.Spec.User
		if strings.Contains(val, "{{") && strings.Contains(val, "}}") {
			val = fmt.Sprintf("%v", eval.Evaluate(val, serverConfig.Config))
		}
		if val != "" {
			args = append(args, "-u", val)
		}
	}

	return args, env
}

func ArgsForPOCI(ctx context.Context, tool catalog.Tool, params any, opts Options, networks []string) []string {
	args := baseArgs(tool.Name, opts)

	// Attach the MCP servers to the same network as the gateway.
	for _, network := range networks {
		args = append(args, "--network", network)
	}

	// Convert params to map[string]any
	arguments, ok := params.(map[string]any)
	if !ok {
		arguments = make(map[string]any)
	}

	// Volumes
	for _, mount := range eval.EvaluateList(tool.Container.Volumes, arguments) {
		if mount == "" {
			continue
		}

		args = append(args, "-v", mount)
	}

	// User
	if tool.Container.User != "" {
		userVal := fmt.Sprintf("%v", eval.Evaluate(tool.Container.User, arguments))
		if userVal != "" {
			args = append(args, "-u", userVal)
		}
	}

	// Image
	args = append(args, tool.Container.Image)

	// Command
	command := eval.EvaluateList(tool.Container.Command, arguments)
	args = append(args, command...)

	return args
}

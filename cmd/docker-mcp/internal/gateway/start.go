package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

var readOnly = true

func (g *Gateway) baseArgs(name string) []string {
	args := []string{"run"}

	// Should we keep the container after it exits? Useful for debugging.
	if !g.KeepContainers {
		args = append(args, "--rm")
	}

	args = append(args, "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", fmt.Sprintf("%d", g.Cpus), "--memory", g.Memory, "--pull", "never")

	if os.Getenv("DOCKER_MCP_IN_DIND") == "1" {
		args = append(args, "--privileged")
	}

	// Add a few labels to the container for identification
	args = append(args,
		"--label", "docker-mcp=true",
		"--label", "docker-mcp-tool-type=mcp",
		"--label", "docker-mcp-name="+name,
		"--label", "docker-mcp-transport=stdio",
	)

	return args
}

func (g *Gateway) runToolContainer(ctx context.Context, tool catalog.Tool, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := g.baseArgs(tool.Name)

	// Attach the MCP servers to the same network as the gateway.
	for _, network := range g.networks {
		args = append(args, "--network", network)
	}

	// Volumes
	for _, mount := range eval.EvaluateList(tool.Container.Volumes, request.GetArguments()) {
		if mount == "" {
			continue
		}

		args = append(args, "-v", mount)
	}

	// Image
	args = append(args, tool.Container.Image)

	// Command
	command := eval.EvaluateList(tool.Container.Command, request.GetArguments())
	args = append(args, command...)

	log("  - Running container", tool.Container.Image, "with args", args)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if g.Verbose {
		cmd.Stderr = os.Stderr
	}
	out, err := cmd.Output()
	if err != nil {
		return mcp.NewToolResultError(string(out)), nil
	}

	return mcp.NewToolResultText(string(out)), nil
}

func (g *Gateway) startMCPClient(ctx context.Context, serverConfig ServerConfig, readOnly *bool) (mcpclient.Client, error) {
	cleanup := func(context.Context) error { return nil }

	var client mcpclient.Client
	var targetConfig proxies.TargetConfig

	if serverConfig.Spec.SSEEndpoint != "" {
		client = mcpclient.NewSSEClient(serverConfig.Name, serverConfig.Spec.SSEEndpoint)
	} else {
		image := serverConfig.Spec.Image
		if g.BlockNetwork && len(serverConfig.Spec.AllowHosts) > 0 {
			var err error
			if targetConfig, cleanup, err = g.runProxies(ctx, serverConfig.Spec.AllowHosts); err != nil {
				return nil, err
			}
		}

		args, env := g.argsAndEnv(serverConfig, readOnly, targetConfig)

		command := expandEnvList(eval.EvaluateList(serverConfig.Spec.Command, serverConfig.Config), env)
		if len(command) == 0 {
			log("  - Running server", imageBaseName(image), "with", args)
		} else {
			log("  - Running server", imageBaseName(image), "with", args, "and command", command)
		}

		var runArgs []string
		runArgs = append(runArgs, args...)
		runArgs = append(runArgs, image)
		runArgs = append(runArgs, command...)

		client = mcpclient.NewStdioCmdClient(serverConfig.Name, "docker", env, runArgs...)
	}

	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "docker",
		Version: "1.0.0",
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx, initRequest, g.Verbose); err != nil {
		initializedObject := serverConfig.Spec.Image
		if serverConfig.Spec.SSEEndpoint != "" {
			initializedObject = serverConfig.Spec.SSEEndpoint
		}
		return nil, fmt.Errorf("initializing %s: %w", initializedObject, err)
	}

	return newClientWithCleanup(client, cleanup), nil
}

func (g *Gateway) argsAndEnv(serverConfig ServerConfig, readOnly *bool, targetConfig proxies.TargetConfig) ([]string, []string) {
	args := g.baseArgs(serverConfig.Name)
	var env []string

	// Security options
	if serverConfig.Spec.DisableNetwork {
		args = append(args, "--network", "none")
	} else {
		// Attach the MCP servers to the same network as the gateway.
		for _, network := range g.networks {
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
		env = append(env, fmt.Sprintf("%s=%s", s.Env, serverConfig.Secrets[s.Name]))
	}

	// Env
	for _, e := range serverConfig.Spec.Env {
		args = append(args, "-e", e.Name)

		value := e.Value
		if strings.Contains(e.Value, "{{") && strings.Contains(e.Value, "}}") {
			value = fmt.Sprintf("%v", eval.Evaluate(value, serverConfig.Config))
		} else {
			value = expandEnv(value, env)
		}

		env = append(env, fmt.Sprintf("%s=%s", e.Name, value))
	}

	// Volumes
	for _, mount := range eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config) {
		if mount == "" {
			continue
		}

		if readOnly != nil && *readOnly {
			args = append(args, "-v", mount+":ro")
		} else {
			args = append(args, "-v", mount)
		}
	}

	return args, env
}

func expandEnv(value string, env []string) string {
	return os.Expand(value, func(name string) string {
		for _, e := range env {
			if after, ok := strings.CutPrefix(e, name+"="); ok {
				return after
			}
		}
		return ""
	})
}

func expandEnvList(values []string, env []string) []string {
	var expanded []string
	for _, value := range values {
		expanded = append(expanded, expandEnv(value, env))
	}
	return expanded
}

package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/catalog"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/eval"
	mcpclient "github.com/docker/docker-mcp/cmd/docker-mcp/internal/mcp"
)

var readOnly = true

func (g *Gateway) baseArgs(name string) []string {
	args := []string{"run"}

	// Should we keep the container after it exits? Useful for debugging.
	if !g.KeepContainers {
		args = append(args, "--rm")
	}

	args = append(args, "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", "1", "--memory", "2Gb", "--pull", "never")

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

	// Volumes
	for _, v := range eval.EvaluateList(tool.Container.Volumes, request.GetArguments()) {
		args = append(args, "-v", v)
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
	if serverConfig.Spec.SSEEndpoint != "" {
		client = mcpclient.NewSSEClient(serverConfig.Name, serverConfig.Spec.SSEEndpoint)
	} else {
		image := serverConfig.Spec.Image

		var network string
		if g.BlockNetwork && len(serverConfig.Spec.AllowHosts) > 0 {
			removeSidecar, internalNetwork, err := g.runProxySideCar(ctx, serverConfig.Spec.AllowHosts)
			if err != nil {
				return nil, err
			}
			cleanup = removeSidecar
			network = internalNetwork
		}

		args, env := g.argsAndEnv(serverConfig, readOnly, network)

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

func (g *Gateway) argsAndEnv(serverConfig ServerConfig, readOnly *bool, proxyNetwork string) ([]string, []string) {
	args := g.baseArgs(serverConfig.Name)
	var env []string

	// Security options
	if serverConfig.Spec.DisableNetwork {
		args = append(args, "--network", "none")
	}
	if proxyNetwork != "" {
		args = append(args, "--network", proxyNetwork)
		args = append(args, "-e", "http_proxy=proxy:8080")
		args = append(args, "-e", "https_proxy=proxy:8080")
	}

	// Secrets
	for _, s := range serverConfig.Spec.Secrets {
		args = append(args, "-e", s.Env)
		env = append(env, fmt.Sprintf("%s=%s", s.Env, serverConfig.Secrets[s.Name]))
	}

	// Env
	for _, e := range serverConfig.Spec.Env {
		args = append(args, "-e", e.Name)
		env = append(env, fmt.Sprintf("%s=%s", e.Name, expandEnv(e.Value, env)))
	}

	// Volumes
	for _, mount := range eval.EvaluateList(serverConfig.Spec.Volumes, serverConfig.Config) {
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
			if strings.HasPrefix(e, name+"=") {
				return strings.TrimPrefix(e, name+"=")
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

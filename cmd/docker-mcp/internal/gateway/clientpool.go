package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/catalog"
	"github.com/docker/docker-mcp/cmd/docker-mcp/internal/eval"
	mcpclient "github.com/docker/docker-mcp/cmd/docker-mcp/internal/mcp"
	"github.com/mark3labs/mcp-go/mcp"
)

// TODO: This is a migrated file from start.go but doesn't have all the updates in it that have been added the last couple weeks.

var readOnly = true

type clientPool struct {
	keepContainers bool
	verbose        bool
	keptClients    sync.Map
}

func newClientPool(keepContainers bool, verbose bool) *clientPool {
	return &clientPool{
		keepContainers: keepContainers,
		verbose:        verbose,
	}
}

func (cp *clientPool) AcquireClient(ctx context.Context, serverConfig ServerConfig, readOnly *bool) (mcpclient.Client, error) {
	var client mcpclient.Client

	if serverConfig.Spec.Stateful {
		if client, ok := cp.keptClients.Load(serverConfig.Name); ok {
			return client.(mcpclient.Client), nil
		}
	}

	if serverConfig.Spec.SSEEndpoint != "" {
		client = mcpclient.NewSSEClient(serverConfig.Name, serverConfig.Spec.SSEEndpoint)
	} else {
		image := serverConfig.Spec.Image

		args, env := cp.argsAndEnv(serverConfig, readOnly)

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

	if _, err := client.Initialize(ctx, initRequest, cp.verbose); err != nil {
		initializedObject := serverConfig.Spec.Image
		if serverConfig.Spec.SSEEndpoint != "" {
			initializedObject = serverConfig.Spec.SSEEndpoint
		}
		return nil, fmt.Errorf("initializing %s: %w", initializedObject, err)
	}

	if serverConfig.Spec.Stateful {
		cp.keptClients.Store(serverConfig.Name, client)
	}

	return client, nil
}

func (cp *clientPool) ReleaseClient(client mcpclient.Client) {
	// TODO how can we lookup by value efficiently?
	// TODO another data structure?
	client.Close()
}

func (cp *clientPool) Close() {
	cp.keptClients.Range(func(key, value any) bool {
		value.(mcpclient.Client).Close()
		return true
	})
	cp.keptClients.Clear()
}

func (cp *clientPool) runToolContainer(ctx context.Context, tool catalog.Tool, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := cp.baseArgs(tool.Name)

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
	if cp.verbose {
		cmd.Stderr = os.Stderr
	}
	out, err := cmd.Output()
	if err != nil {
		return mcp.NewToolResultError(string(out)), nil
	}

	return mcp.NewToolResultText(string(out)), nil
}

func (cp *clientPool) baseArgs(name string) []string {
	args := []string{"run"}

	// Should we keep the container after it exits? Useful for debugging.
	if !cp.keepContainers {
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

func (cp *clientPool) argsAndEnv(serverConfig ServerConfig, readOnly *bool) ([]string, []string) {
	args := cp.baseArgs(serverConfig.Name)
	var env []string

	// Security options
	if serverConfig.Spec.DisableNetwork {
		args = append(args, "--network", "none")
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

package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

var readOnly = true

type keptClient struct {
	Name   string
	Getter *clientGetter
	Config ServerConfig
}

type clientPool struct {
	Options
	keptClients []keptClient
	clientLock  sync.RWMutex
	networks    []string
	docker      docker.Client
}

func newClientPool(options Options, docker docker.Client) *clientPool {
	return &clientPool{
		Options:     options,
		docker:      docker,
		keptClients: []keptClient{},
	}
}

func (cp *clientPool) AcquireClient(ctx context.Context, serverConfig ServerConfig, readOnly *bool) (mcpclient.Client, error) {
	var getter *clientGetter

	// Check if client is kept, can be returned immediately
	cp.clientLock.RLock()
	for _, kc := range cp.keptClients {
		if kc.Name == serverConfig.Name {
			getter = kc.Getter
			break
		}
	}
	cp.clientLock.RUnlock()

	// No client found, create a new one
	if getter == nil {
		getter = newClientGetter(serverConfig, cp, readOnly)

		// If the client is long running, save it for later
		if serverConfig.Spec.Singleton || cp.AllSingletons {
			cp.clientLock.Lock()
			cp.keptClients = append(cp.keptClients, keptClient{
				Name:   serverConfig.Name,
				Getter: getter,
				Config: serverConfig,
			})
			cp.clientLock.Unlock()
		}
	}

	client, err := getter.GetClient(ctx) // first time creates the client, can take some time
	if err != nil {
		cp.clientLock.Lock()
		defer cp.clientLock.Unlock()

		// Wasn't successful, remove it
		if serverConfig.Spec.Singleton || cp.AllSingletons {
			for i, kc := range cp.keptClients {
				if kc.Getter == getter {
					cp.keptClients = append(cp.keptClients[:i], cp.keptClients[i+1:]...)
					break
				}
			}
		}

		return nil, err
	}

	return client, nil
}

func (cp *clientPool) ReleaseClient(client mcpclient.Client) {
	foundKept := false
	cp.clientLock.RLock()
	for _, kc := range cp.keptClients {
		if kc.Getter.IsClient(client) {
			foundKept = true
			break
		}
	}
	cp.clientLock.RUnlock()

	// Client was not kept, close it
	if !foundKept {
		client.Close()
		return
	}

	// Otherwise, leave the client as is
}

func (cp *clientPool) Close() {
	cp.clientLock.Lock()
	existingMap := cp.keptClients
	cp.keptClients = []keptClient{}
	cp.clientLock.Unlock()

	// Close all clients
	for _, keptClient := range existingMap {
		client, err := keptClient.Getter.GetClient(context.TODO()) // should be cached
		if err == nil {
			client.Close()
		}
	}
}

func (cp *clientPool) SetNetworks(networks []string) {
	cp.networks = networks
}

func (cp *clientPool) runToolContainer(ctx context.Context, tool catalog.Tool, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := cp.baseArgs(tool.Name)

	// Attach the MCP servers to the same network as the gateway.
	for _, network := range cp.networks {
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
	if cp.Verbose {
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

	args = append(args, "--rm", "-i", "--init", "--security-opt", "no-new-privileges", "--cpus", fmt.Sprintf("%d", cp.Cpus), "--memory", cp.Memory, "--pull", "never")

	if os.Getenv("DOCKER_MCP_IN_DIND") == "1" {
		args = append(args, "--privileged")
	}

	// Add a few labels to the container for identification
	args = append(args,
		"-l", "docker-mcp=true",
		"-l", "docker-mcp-tool-type=mcp",
		"-l", "docker-mcp-name="+name,
		"-l", "docker-mcp-transport=stdio",
	)

	return args
}

func (cp *clientPool) argsAndEnv(serverConfig ServerConfig, readOnly *bool, targetConfig proxies.TargetConfig) ([]string, []string) {
	args := cp.baseArgs(serverConfig.Name)
	var env []string

	// Security options
	if serverConfig.Spec.DisableNetwork {
		args = append(args, "--network", "none")
	} else {
		// Attach the MCP servers to the same network as the gateway.
		for _, network := range cp.networks {
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
			env = append(env, fmt.Sprintf("%s=%s", s.Env, "<UNKNOWN>"))
		}
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

type clientGetter struct {
	once   sync.Once
	client mcpclient.Client
	err    error

	serverConfig ServerConfig
	cp           *clientPool
	readOnly     *bool
}

func newClientGetter(serverConfig ServerConfig, cp *clientPool, readOnly *bool) *clientGetter {
	return &clientGetter{
		serverConfig: serverConfig,
		cp:           cp,
		readOnly:     readOnly,
	}
}

func (cg *clientGetter) IsClient(client mcpclient.Client) bool {
	return cg.client == client
}

func (cg *clientGetter) GetClient(ctx context.Context) (mcpclient.Client, error) {
	cg.once.Do(func() {
		createClient := func() (mcpclient.Client, error) {
			cleanup := func(context.Context) error { return nil }

			var client mcpclient.Client

			if cg.serverConfig.Spec.SSEEndpoint != "" {
				client = mcpclient.NewSSEClient(cg.serverConfig.Name, cg.serverConfig.Spec.SSEEndpoint)
			} else if cg.cp.Static {
				client = mcpclient.NewStdioCmdClient(cg.serverConfig.Name, "socat", nil, "STDIO", fmt.Sprintf("TCP:mcp-%s:4444", cg.serverConfig.Name))
			} else {
				var targetConfig proxies.TargetConfig
				if cg.cp.BlockNetwork && len(cg.serverConfig.Spec.AllowHosts) > 0 {
					var err error
					if targetConfig, cleanup, err = cg.cp.runProxies(ctx, cg.serverConfig.Spec.AllowHosts, cg.serverConfig.Spec.Singleton); err != nil {
						return nil, err
					}
				}

				image := cg.serverConfig.Spec.Image
				args, env := cg.cp.argsAndEnv(cg.serverConfig, cg.readOnly, targetConfig)

				command := expandEnvList(eval.EvaluateList(cg.serverConfig.Spec.Command, cg.serverConfig.Config), env)
				if len(command) == 0 {
					log("  - Running", imageBaseName(image), "with", args)
				} else {
					log("  - Running", imageBaseName(image), "with", args, "and command", command)
				}

				var runArgs []string
				runArgs = append(runArgs, args...)
				runArgs = append(runArgs, image)
				runArgs = append(runArgs, command...)

				client = mcpclient.NewStdioCmdClient(cg.serverConfig.Name, "docker", env, runArgs...)
			}

			initRequest := mcp.InitializeRequest{}
			initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
			initRequest.Params.ClientInfo = mcp.Implementation{
				Name:    "docker",
				Version: "1.0.0",
			}

			ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			defer cancel()

			if _, err := client.Initialize(ctx, initRequest, cg.cp.Verbose); err != nil {
				return nil, err
			}

			return newClientWithCleanup(client, cleanup), nil
		}

		client, err := createClient()
		cg.client = client
		cg.err = err
	})

	return cg.client, cg.err
}

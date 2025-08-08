package gateway

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

type keptClient struct {
	Name   string
	Getter *clientGetter
	Config catalog.ServerConfig
}

type clientPool struct {
	Options
	keptClients []keptClient
	clientLock  sync.RWMutex
	networks    []string
	docker      docker.Client
}

type clientConfig struct {
	readOnly      *bool
	serverSession *mcp.ServerSession
	server        *mcp.Server
}

func newClientPool(options Options, docker docker.Client) *clientPool {
	return &clientPool{
		Options:     options,
		docker:      docker,
		keptClients: []keptClient{},
	}
}

func (cp *clientPool) UpdateRoots(ss *mcp.ServerSession, roots []*mcp.Root) {
	cp.clientLock.RLock()
	defer cp.clientLock.RUnlock()

	for _, kc := range cp.keptClients {
		client, err := kc.Getter.GetClient(context.TODO()) // should be cached
		if err == nil {
			client.GetClient().AddRoots(roots...)
		}
	}
}

func (cp *clientPool) longLived(serverConfig catalog.ServerConfig, config *clientConfig) bool {
	keep := config != nil && config.serverSession != nil && (serverConfig.Spec.LongLived || cp.LongLived)
	return keep
}

func (cp *clientPool) AcquireClient(ctx context.Context, serverConfig catalog.ServerConfig, config *clientConfig) (mcpclient.Client, error) {
	var getter *clientGetter
	c := ctx

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
		getter = newClientGetter(serverConfig, cp, config)

		// If the client is long running, save it for later
		if cp.longLived(serverConfig, config) {
			c = context.Background()
			cp.clientLock.Lock()
			cp.keptClients = append(cp.keptClients, keptClient{
				Name:   serverConfig.Name,
				Getter: getter,
				Config: serverConfig,
			})
			cp.clientLock.Unlock()
		}
	}

	client, err := getter.GetClient(c) // first time creates the client, can take some time
	if err != nil {
		cp.clientLock.Lock()
		defer cp.clientLock.Unlock()

		// Wasn't successful, remove it
		if cp.longLived(serverConfig, config) {
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
		client.Session().Close()
		return
	}
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
			client.Session().Close()
		}
	}
}

func (cp *clientPool) SetNetworks(networks []string) {
	cp.networks = networks
}

func (cp *clientPool) runToolContainer(ctx context.Context, tool catalog.Tool, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	args := cp.baseArgs(tool.Name)

	// Attach the MCP servers to the same network as the gateway.
	for _, network := range cp.networks {
		args = append(args, "--network", network)
	}

	// Convert params.Arguments to map[string]any
	arguments, ok := params.Arguments.(map[string]any)
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

	// Image
	args = append(args, tool.Container.Image)

	// Command
	command := eval.EvaluateList(tool.Container.Command, arguments)
	args = append(args, command...)

	log("  - Running container", tool.Container.Image, "with args", args)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if cp.Verbose {
		cmd.Stderr = os.Stderr
	}
	out, err := cmd.Output()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: string(out),
			}},
			IsError: true,
		}, nil
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: string(out),
		}},
		IsError: false,
	}, nil
}

func (cp *clientPool) baseArgs(name string) []string {
	args := []string{"run"}

	args = append(args, "--rm", "-i", "--init", "--security-opt", "no-new-privileges")
	if cp.Cpus > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%d", cp.Cpus))
	}
	if cp.Memory != "" {
		args = append(args, "--memory", cp.Memory)
	}
	args = append(args, "--pull", "never")

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

func (cp *clientPool) argsAndEnv(serverConfig catalog.ServerConfig, readOnly *bool, targetConfig proxies.TargetConfig) ([]string, []string) {
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

	serverConfig catalog.ServerConfig
	cp           *clientPool

	clientConfig *clientConfig
}

func newClientGetter(serverConfig catalog.ServerConfig, cp *clientPool, config *clientConfig) *clientGetter {
	return &clientGetter{
		serverConfig: serverConfig,
		cp:           cp,
		clientConfig: config,
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

			// Deprecated: Use Remote instead
			if cg.serverConfig.Spec.SSEEndpoint != "" {
				client = mcpclient.NewRemoteMCPClient(cg.serverConfig)
			} else if cg.serverConfig.Spec.Remote.URL != "" {
				client = mcpclient.NewRemoteMCPClient(cg.serverConfig)
			} else if cg.cp.Static {
				client = mcpclient.NewStdioCmdClient(cg.serverConfig.Name, "socat", nil, "STDIO", fmt.Sprintf("TCP:mcp-%s:4444", cg.serverConfig.Name))
			} else {
				var targetConfig proxies.TargetConfig
				if cg.cp.BlockNetwork && len(cg.serverConfig.Spec.AllowHosts) > 0 {
					var err error
					if targetConfig, cleanup, err = cg.cp.runProxies(ctx, cg.serverConfig.Spec.AllowHosts, cg.serverConfig.Spec.LongLived); err != nil {
						return nil, err
					}
				}

				image := cg.serverConfig.Spec.Image
				var readOnly *bool
				if cg.clientConfig != nil {
					readOnly = cg.clientConfig.readOnly
				}
				args, env := cg.cp.argsAndEnv(cg.serverConfig, readOnly, targetConfig)

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

			initParams := &mcp.InitializeParams{
				ProtocolVersion: "2024-11-05",
				ClientInfo: &mcp.Implementation{
					Name:    "docker",
					Version: "1.0.0",
				},
			}

			var ss *mcp.ServerSession
			var server *mcp.Server
			if cg.clientConfig != nil {
				ss = cg.clientConfig.serverSession
				server = cg.clientConfig.server
			}
			// ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			// defer cancel()

			if err := client.Initialize(ctx, initParams, cg.cp.Verbose, []*mcp.Root{}, ss, server); err != nil {
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

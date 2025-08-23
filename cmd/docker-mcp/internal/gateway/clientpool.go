package gateway

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/catalog"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/docker"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/eval"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/provisioners"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/proxies"
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/gateway/runtime"
	mcpclient "github.com/docker/mcp-gateway/cmd/docker-mcp/internal/mcp"
)

type clientKey struct {
	serverName string
	session    *mcp.ServerSession
}

type keptClient struct {
	Name         string
	Getter       *clientGetter
	Config       *catalog.ServerConfig
	ClientConfig *clientConfig
}

type clientPool struct {
	Options
	keptClients        map[clientKey]keptClient
	clientLock         sync.RWMutex
	networks           []string
	docker             docker.Client
	gateway            *Gateway
	defaultProvisioner provisioners.ProvisionerType
	provisionerMap     map[provisioners.ProvisionerType]provisioners.Provisioner
	containerRuntime   runtime.ContainerRuntime
}

type clientConfig struct {
	readOnly      *bool
	serverSession *mcp.ServerSession
	server        *mcp.Server
}

// ClientPoolConfig holds the configuration for creating a client pool
type ClientPoolConfig struct {
	Options            Options
	Docker             docker.Client
	Gateway            *Gateway
	ContainerRuntime   runtime.ContainerRuntime
	Provisioners       map[provisioners.ProvisionerType]provisioners.Provisioner
	DefaultProvisioner provisioners.ProvisionerType
}

func newClientPool(config ClientPoolConfig) *clientPool {
	cp := &clientPool{
		Options:            config.Options,
		docker:             config.Docker,
		gateway:            config.Gateway,
		keptClients:        make(map[clientKey]keptClient),
		containerRuntime:   config.ContainerRuntime,
		defaultProvisioner: config.DefaultProvisioner,
		provisionerMap:     make(map[provisioners.ProvisionerType]provisioners.Provisioner),
	}

	// Register all provided provisioners
	for provisionerType, provisioner := range config.Provisioners {
		cp.provisionerMap[provisionerType] = provisioner
	}

	return cp
}

// SetConfigResolver injects a config resolver into the Docker provisioner.
// This should be called after configurations are loaded.
func (cp *clientPool) SetConfigResolver(resolver provisioners.ConfigResolver) error {
	// Only Docker provisioner needs a config resolver
	provisioner, exists := cp.provisionerMap[provisioners.DockerProvisioner]
	if !exists {
		return fmt.Errorf("docker provisioner not found")
	}

	// Type assert to get the Docker provisioner implementation
	dockerProvisioner, ok := provisioner.(*provisioners.DockerProvisionerImpl)
	if !ok {
		return fmt.Errorf("provisioner is not a DockerProvisionerImpl")
	}

	// Inject the resolver (we need to add a SetConfigResolver method to DockerProvisioner)
	dockerProvisioner.SetConfigResolver(resolver)
	return nil
}

// getProvisioner returns the default provisioner for the client pool.
// The provisioner type is determined at deployment time, not per-server.
func (cp *clientPool) getProvisioner() (provisioners.Provisioner, error) {
	// Look up the provisioner in the map
	provisioner, exists := cp.provisionerMap[cp.defaultProvisioner]
	if !exists {
		return nil, fmt.Errorf("provisioner type %s not available", cp.defaultProvisioner.String())
	}

	return provisioner, nil
}

// SetProvisioner sets a provisioner implementation for a specific type
func (cp *clientPool) SetProvisioner(provisionerType provisioners.ProvisionerType, provisioner provisioners.Provisioner) {
	cp.provisionerMap[provisionerType] = provisioner
}

// SetDefaultProvisioner sets the default provisioner type
func (cp *clientPool) SetDefaultProvisioner(provisionerType provisioners.ProvisionerType) {
	cp.defaultProvisioner = provisionerType
}

func (cp *clientPool) UpdateRoots(ss *mcp.ServerSession, roots []*mcp.Root) {
	cp.clientLock.RLock()
	defer cp.clientLock.RUnlock()

	for _, kc := range cp.keptClients {
		if kc.ClientConfig != nil && (kc.ClientConfig.serverSession == ss) {
			client, err := kc.Getter.GetClient(context.TODO()) // should be cached
			if err == nil {
				client.AddRoots(roots)
			}
		}
	}
}

func (cp *clientPool) longLived(serverConfig *catalog.ServerConfig, config *clientConfig) bool {
	keep := config != nil && config.serverSession != nil && (serverConfig.Spec.LongLived || cp.LongLived)
	return keep
}

func (cp *clientPool) AcquireClient(ctx context.Context, serverConfig *catalog.ServerConfig, config *clientConfig) (mcpclient.Client, error) {
	var getter *clientGetter
	c := ctx

	// Check if client is kept, can be returned immediately
	var session *mcp.ServerSession
	if config != nil {
		session = config.serverSession
	}
	key := clientKey{serverName: serverConfig.Name, session: session}
	cp.clientLock.RLock()
	if kc, exists := cp.keptClients[key]; exists {
		getter = kc.Getter
	}
	cp.clientLock.RUnlock()

	// No client found, create a new one
	if getter == nil {
		getter = newClientGetter(serverConfig, cp, config)

		// If the client is long running, save it for later
		if cp.longLived(serverConfig, config) {
			c = context.Background()
			cp.clientLock.Lock()
			cp.keptClients[key] = keptClient{
				Name:         serverConfig.Name,
				Getter:       getter,
				Config:       serverConfig,
				ClientConfig: config,
			}
			cp.clientLock.Unlock()
		}
	}

	client, err := getter.GetClient(c) // first time creates the client, can take some time
	if err != nil {
		cp.clientLock.Lock()
		defer cp.clientLock.Unlock()

		// Wasn't successful, remove it
		if cp.longLived(serverConfig, config) {
			delete(cp.keptClients, key)
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
		// Check if this is a client with cleanup (ephemeral containers)
		if cleanupClient, ok := client.(*clientWithCleanup); ok {
			// Call cleanup function first (stops container)
			if cleanup := cleanupClient.GetCleanup(); cleanup != nil {
				if err := cleanup(context.Background()); err != nil {
					log("Warning: Error during ephemeral container cleanup:", err)
				}
			}
		}
		client.Session().Close()
		return
	}
}

func (cp *clientPool) Close() {
	cp.clientLock.Lock()
	existingMap := cp.keptClients
	cp.keptClients = make(map[clientKey]keptClient)
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

	// Update the DockerProvisioner's networks configuration if it exists
	if dockerProvisioner, exists := cp.provisionerMap[provisioners.DockerProvisioner]; exists {
		if dp, ok := dockerProvisioner.(*provisioners.DockerProvisionerImpl); ok {
			dp.SetNetworks(networks)
		}
	}
}

func (cp *clientPool) runToolContainer(ctx context.Context, tool catalog.Tool, params *mcp.CallToolParams) (*mcp.CallToolResult, error) {
	log("=== runToolContainer ENTRY for tool:", tool.Name, "===")
	log("Tool image:", tool.Container.Image)
	log("Tool command template:", tool.Container.Command)

	// Convert params.Arguments to map[string]any
	arguments, ok := params.Arguments.(map[string]any)
	if !ok {
		arguments = make(map[string]any)
	}
	log("Tool arguments:", arguments)

	// Get provisioner for secret/config provider support (same as MCP servers)
	log("Getting provisioner for tool execution...")
	provisioner, err := cp.getProvisioner()
	if err != nil {
		log("ERROR: Failed to get provisioner:", err)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Failed to get provisioner for tool execution: %v", err),
			}},
			IsError: true,
		}, nil
	}
	log("SUCCESS: Got provisioner:", fmt.Sprintf("%T", provisioner))

	// Build basic environment variables from tool definition
	env := make(map[string]string)
	// TODO: Add support for tool.Container.Env when Container struct is extended

	// Build container specification from tool definition
	spec := runtime.ContainerSpec{
		Name:     tool.Name,
		Image:    tool.Container.Image,
		Command:  eval.EvaluateList(tool.Container.Command, arguments),
		Networks: cp.networks, // Use networks from clientPool
		Volumes:  []string{},  // Will be populated below
		Env:      env,

		// Container behavior (matching existing baseArgs logic)
		RemoveAfterRun: true,  // --rm
		Interactive:    true,  // -i
		Init:           true,  // --init
		Privileged:     false, // No privileged mode for tools

		// Resource limits (from clientPool options)
		CPUs:   cp.Cpus,
		Memory: cp.Memory,

		// Network isolation
		DisableNetwork: false, // Tools should have network access by default
	}

	// Apply secret/config provider support using generic provisioner interface
	// This uses the same docker-engine vs cluster provider logic as MCP servers
	provisioner.ApplyToolProviders(&spec, tool.Name)

	// Process volumes with template evaluation
	for _, mount := range eval.EvaluateList(tool.Container.Volumes, arguments) {
		if mount != "" {
			spec.Volumes = append(spec.Volumes, mount)
		}
	}

	// Handle User setting with template evaluation
	if tool.Container.User != "" {
		userVal := fmt.Sprintf("%v", eval.Evaluate(tool.Container.User, arguments))
		if userVal != "" {
			spec.User = userVal
		}
	}

	// Log container execution (similar to existing log)
	if len(spec.Command) == 0 {
		log("  - Running container", spec.Image, "via ContainerRuntime")
	} else {
		log("  - Running container", spec.Image, "with command", spec.Command, "via ContainerRuntime")
	}
	log("Container spec details:")
	log("  - Name:", spec.Name)
	log("  - Image:", spec.Image)
	log("  - Command:", spec.Command)
	log("  - Env vars:", len(spec.Env))
	log("  - Volumes:", len(spec.Volumes))
	log("  - CPUs:", spec.CPUs)
	log("  - Memory:", spec.Memory)

	// Execute container using Container Runtime
	log("Calling containerRuntime.RunContainer...")
	result, err := cp.containerRuntime.RunContainer(ctx, spec)
	if err != nil {
		log("ERROR: containerRuntime.RunContainer failed:", err)
		// Container runtime error (not container execution failure)
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{
				Text: fmt.Sprintf("Container runtime error: %v", err),
			}},
			IsError: true,
		}, nil
	}

	log("SUCCESS: containerRuntime.RunContainer completed")
	log("Result details:")
	log("  - Exit code:", result.ExitCode)
	log("  - Success:", result.Success)
	log("  - Stdout length:", len(result.Stdout))
	log("  - Container ID:", result.ContainerID)
	log("  - Runtime:", result.Runtime)

	if len(result.Stdout) > 100 {
		log("  - Stdout preview:", result.Stdout[:100])
	} else {
		log("  - Full stdout:", result.Stdout)
	}

	// Return result based on container execution
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{
			Text: result.Stdout,
		}},
		IsError: !result.Success, // Use container's success status
	}, nil
}

// Deprecated: baseArgs is only used by legacy tests.
// All client creation now goes through provisioner interface.
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

// Deprecated: argsAndEnv is only used by legacy tests.
// All client creation now goes through provisioner interface.
func (cp *clientPool) argsAndEnv(serverConfig *catalog.ServerConfig, readOnly *bool, targetConfig proxies.TargetConfig) ([]string, []string) {
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

// Deprecated: expandEnv is only used by legacy tests and argsAndEnv.
// All client creation now goes through provisioner interface.
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

type clientGetter struct {
	once   sync.Once
	client mcpclient.Client
	err    error

	serverConfig *catalog.ServerConfig
	cp           *clientPool

	clientConfig *clientConfig
}

func newClientGetter(serverConfig *catalog.ServerConfig, cp *clientPool, config *clientConfig) *clientGetter {
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
			// Get provisioner - all client creation now goes through provisioner interface
			provisioner, err := cg.cp.getProvisioner()
			if err != nil {
				return nil, fmt.Errorf("failed to get provisioner: %w", err)
			}

			// Use provisioner interface
			spec, err := provisioners.AdaptServerConfigToSpec(cg.serverConfig, cg.cp.defaultProvisioner)
			if err != nil {
				return nil, fmt.Errorf("failed to adapt server config to provisioner spec: %w", err)
			}

			var ss *mcp.ServerSession
			var server *mcp.Server
			if cg.clientConfig != nil {
				ss = cg.clientConfig.serverSession
				server = cg.clientConfig.server
			}
			// ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
			// defer cancel()

			// TODO add initial roots
			//if err := client.Initialize(ctx, initParams, cg.cp.Verbose, ss, server, cg.cp.gateway); err != nil {
				//return nil, err
			//}
			// Pre-validate deployment
			if err := provisioner.PreValidateDeployment(ctx, spec); err != nil {
				return nil, fmt.Errorf("pre-validation failed: %w", err)
			}

			// Provision the server
			client, cleanup, err := provisioner.ProvisionServer(ctx, spec, ss, server)
			if err != nil {
				return nil, fmt.Errorf("provisioning failed: %w", err)
			}

			return newClientWithCleanup(client, func(_ context.Context) error {
				cleanup()
				return nil
			}), nil
		}

		client, err := createClient()
		cg.client = client
		cg.err = err
	})

	return cg.client, cg.err
}

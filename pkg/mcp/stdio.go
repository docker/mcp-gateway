package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/logs"
)

type stdioMCPClient struct {
	name        string
	command     string
	env         []string
	args        []string
	client      *mcp.Client
	session     *mcp.ClientSession
	roots       []*mcp.Root
	initialized atomic.Bool
}

func NewStdioCmdClient(name string, command string, env []string, args ...string) Client {
	return &stdioMCPClient{
		name:    name,
		command: command,
		env:     env,
		args:    args,
	}
}

func (c *stdioMCPClient) Initialize(ctx context.Context, _ *mcp.InitializeParams, debug bool, ss *mcp.ServerSession, server *mcp.Server, refresher CapabilityRefresher) error {
	if c.initialized.Load() {
		return fmt.Errorf("client already initialized")
	}

	cmd := exec.CommandContext(ctx, c.command, c.args...)
	cmd.Env = commandEnv(c.env)

	if debug {
		cmd.Stderr = logs.NewPrefixer(os.Stderr, "- "+c.name+": ")
	}

	transport := &mcp.CommandTransport{Command: cmd}
	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    "docker-mcp-gateway",
		Version: "1.0.0",
	}, notifications(c.name, ss, server, refresher))

	c.client.AddRoots(c.roots...)

	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.session = session
	c.initialized.Store(true)

	return nil
}

// commandEnv builds the environment for the MCP server's `docker` subprocess.
//
// The subprocess must be able to resolve the *same* Docker endpoint the gateway
// resolved. Forwarding only PATH happens to work on Linux/macOS (the default
// unix:///var/run/docker.sock exists there), but on Windows the docker CLI
// falls back to npipe:////./pipe/docker_engine -- a named pipe that only Docker
// Desktop creates. So on Docker Engine for Windows (no Desktop) every server
// fails with:
//
//	failed to connect to the docker API at npipe:////./pipe/docker_engine
//
// Forward the Docker client variables plus the OS essentials the CLI needs to
// locate its configuration, then let the server's own env override them.
var inheritedEnvKeys = []string{
	"PATH",
	// Docker endpoint / client configuration.
	"DOCKER_HOST",
	"DOCKER_CONTEXT",
	"DOCKER_CONFIG",
	"DOCKER_TLS_VERIFY",
	"DOCKER_CERT_PATH",
	// The CLI resolves its config directory from the user profile. On Windows
	// that is USERPROFILE (HOME/HOMEDRIVE/HOMEPATH are the POSIX fallbacks).
	"HOME",
	"USERPROFILE",
	"HOMEDRIVE",
	"HOMEPATH",
	"SystemRoot",
	"APPDATA",
	"LOCALAPPDATA",
}

func commandEnv(env []string) []string {
	merged := env
	for _, key := range inheritedEnvKeys {
		if envHasKey(merged, key) {
			continue
		}
		if value := os.Getenv(key); value != "" {
			merged = append(merged, key+"="+value)
		}
	}
	return merged
}

func envHasKey(env []string, key string) bool {
	prefix := key + "="
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			return true
		}
	}
	return false
}

func (c *stdioMCPClient) AddRoots(roots []*mcp.Root) {
	if c.initialized.Load() {
		c.client.AddRoots(roots...)
	}
	c.roots = roots
}

func (c *stdioMCPClient) Session() *mcp.ClientSession {
	if !c.initialized.Load() {
		panic("client not initialize")
	}
	return c.session
}

func (c *stdioMCPClient) GetClient() *mcp.Client {
	if !c.initialized.Load() {
		panic("client not initialize")
	}
	return c.client
}

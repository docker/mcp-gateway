package mcp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync/atomic"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/logs"
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
	cmd.Env = c.env

	if debug {
		cmd.Stderr = logs.NewPrefixer(os.Stderr, "- "+c.name+": ")
	}

	transport := &mcp.CommandTransport{Command: cmd}
	c.client = mcp.NewClient(&mcp.Implementation{
		Name:    "docker-mcp-gateway",
		Version: "1.0.0",
	}, notifications(ss, server, refresher))

	c.client.AddRoots(c.roots...)

	session, err := c.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}

	c.session = session
	c.initialized.Store(true)

	return nil
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

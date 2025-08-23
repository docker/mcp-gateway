package mcp

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sync/atomic"
	"unsafe"

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

// stdioHandleMCPClient uses existing stdio handles instead of spawning a command
type stdioHandleMCPClient struct {
	name        string
	rwc         io.ReadWriteCloser
	client      *mcp.Client
	session     *mcp.ClientSession
	roots       []*mcp.Root
	initialized atomic.Bool
}

func NewStdioHandleClient(name string, stdin io.WriteCloser, stdout io.ReadCloser) Client {
	// Create a combined ReadWriteCloser from the separate handles
	rwc := &stdioHandleRWC{
		stdin:  stdin,
		stdout: stdout,
	}

	return &stdioHandleMCPClient{
		name: name,
		rwc:  rwc,
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

func (c *stdioHandleMCPClient) Initialize(ctx context.Context, _ *mcp.InitializeParams, _ bool, ss *mcp.ServerSession, server *mcp.Server, refresher CapabilityRefresher) error {
	if c.initialized.Load() {
		return fmt.Errorf("client already initialized")
	}

	// Create a transport using existing handles
	// We need to create an ioTransport but it's not exported
	// Use reflection to create it similar to how InMemoryTransport does it
	transport := createIOTransport(c.rwc)

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

func (c *stdioHandleMCPClient) AddRoots(roots []*mcp.Root) {
	if c.initialized.Load() {
		c.client.AddRoots(roots...)
	}
	c.roots = roots
}

func (c *stdioHandleMCPClient) Session() *mcp.ClientSession {
	if !c.initialized.Load() {
		panic("client not initialize")
	}
	return c.session
}

func (c *stdioHandleMCPClient) GetClient() *mcp.Client {
	if !c.initialized.Load() {
		panic("client not initialize")
	}
	return c.client
}

// stdioHandleRWC implements io.ReadWriteCloser for existing stdio handles
type stdioHandleRWC struct {
	stdin  io.WriteCloser
	stdout io.ReadCloser
}

func (s *stdioHandleRWC) Read(p []byte) (n int, err error) {
	return s.stdout.Read(p)
}

func (s *stdioHandleRWC) Write(p []byte) (n int, err error) {
	return s.stdin.Write(p)
}

func (s *stdioHandleRWC) Close() error {
	// Close both handles
	var err1, err2 error
	if s.stdin != nil {
		err1 = s.stdin.Close()
	}
	if s.stdout != nil {
		err2 = s.stdout.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// createIOTransport creates an ioTransport using reflection
// This is needed because ioTransport is not exported from the MCP SDK
func createIOTransport(rwc io.ReadWriteCloser) mcp.Transport {
	// Create a StdioTransport and then replace its internal rwc
	stdioTransport := mcp.NewStdioTransport()

	// Use reflection to access the internal ioTransport field
	transportValue := reflect.ValueOf(stdioTransport).Elem()
	ioTransportField := transportValue.FieldByName("ioTransport")

	// Access the rwc field within ioTransport
	ioTransportPtr := unsafe.Pointer(ioTransportField.UnsafeAddr())
	ioTransportValue := reflect.NewAt(ioTransportField.Type(), ioTransportPtr).Elem()
	rwcField := ioTransportValue.FieldByName("rwc")

	// Set our custom rwc
	rwcPtr := unsafe.Pointer(rwcField.UnsafeAddr())
	rwcValue := reflect.NewAt(rwcField.Type(), rwcPtr).Elem()
	rwcValue.Set(reflect.ValueOf(rwc))

	return stdioTransport
}

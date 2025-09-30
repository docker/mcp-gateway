package interceptors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/google/shlex"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/docker/mcp-gateway/pkg/logs"
)

func Callbacks(logCalls, blockSecrets bool, oauthInterceptorEnabled bool, interceptors []Interceptor) []mcp.Middleware {
	var middleware []mcp.Middleware

	// Add telemetry middleware (always enabled)
	middleware = append(middleware, TelemetryMiddleware())

	// Add GitHub unauthorized interceptor only if the feature is enabled
	// This ensures GitHub 401 responses are handled with OAuth links when requested
	if oauthInterceptorEnabled {
		middleware = append(middleware, GitHubUnauthorizedMiddleware())
	}

	// Add custom interceptors
	for _, interceptor := range interceptors {
		middleware = append(middleware, interceptor.ToMiddleware())
	}

	// Add log calls middleware
	if logCalls {
		middleware = append(middleware, LogCallsMiddleware())
	}

	// Add block secrets middleware
	if blockSecrets {
		middleware = append(middleware, BlockSecretsMiddleware())
	}

	return middleware
}

type Interceptor struct {
	When     string
	Type     string
	Argument string
}

// --interceptor=before:exec:/bin/path
// --interceptor=after:docker:image
// --interceptor=around:http:localhost:8080/url
func Parse(specs []string) ([]Interceptor, error) {
	var interceptors []Interceptor

	for _, spec := range specs {
		parts := strings.SplitN(spec, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid interceptor spec '%s', expected format is 'when:type:path'", spec)
		}

		w := strings.ToLower(parts[0])
		if w != "before" && w != "after" {
			return nil, fmt.Errorf("invalid interceptor when: '%s', expected 'before' or 'after''", w)
		}

		t := strings.ToLower(parts[1])
		if t != "exec" && t != "docker" && t != "http" {
			return nil, fmt.Errorf("invalid interceptor type: '%s', expected 'exec', 'docker', or 'http'", t)
		}

		interceptors = append(interceptors, Interceptor{
			When:     w,
			Type:     t,
			Argument: parts[2],
		})
	}

	return interceptors, nil
}

func (i *Interceptor) ToMiddleware() mcp.Middleware {
	return func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			// Only intercept tools/call method
			if method != "tools/call" {
				return next(ctx, method, req)
			}

			if i.When == "before" {
				message, err := json.Marshal(req)
				if err != nil {
					return nil, fmt.Errorf("marshalling request: %w", err)
				}

				out, err := i.run(ctx, message)
				if err != nil {
					return nil, fmt.Errorf("executing interceptor: %w", err)
				}

				// If the interceptor returns a response, we use it instead of calling the next handler.
				if len(out) > 0 {
					var result mcp.CallToolResult
					if err := json.Unmarshal(out, &result); err != nil {
						return nil, fmt.Errorf("unmarshalling interceptor response: %w", err)
					}
					return &result, nil
				}
			}

			response, err := next(ctx, method, req)

			if i.When == "after" {
				message, err := json.Marshal(response)
				if err != nil {
					return nil, fmt.Errorf("marshalling response: %w", err)
				}

				out, err := i.run(ctx, message)
				if err != nil {
					return nil, fmt.Errorf("executing interceptor: %w", err)
				}

				// If the interceptor returns a response, we use it instead.
				if len(out) > 0 {
					var result mcp.CallToolResult
					if err := json.Unmarshal(out, &result); err != nil {
						return nil, fmt.Errorf("unmarshalling interceptor response: %w", err)
					}
					return &result, nil
				}
			}

			return response, err
		}
	}
}

func (i *Interceptor) run(ctx context.Context, message []byte) ([]byte, error) {
	switch i.Type {
	case "exec":
		return i.runExec(ctx, message)
	case "docker":
		return i.runDocker(ctx, message)
	case "http":
		return i.runHTTP(ctx, message)
	}

	return nil, fmt.Errorf("unknown interceptor type '%s'", i.Type)
}

func (i *Interceptor) runExec(ctx context.Context, message []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", i.Argument)
	cmd.Stdin = bytes.NewBuffer(message)
	cmd.Stderr = logs.NewPrefixer(os.Stderr, "  - ")
	return cmd.Output()
}

func (i *Interceptor) runDocker(ctx context.Context, message []byte) ([]byte, error) {
	image, rest, _ := strings.Cut(i.Argument, " ")

	args := []string{"run", "--rm", "--init", image}
	if len(rest) > 0 {
		moreArgs, err := shlex.Split(rest)
		if err != nil {
			return nil, fmt.Errorf("parsing docker arguments: %w", err)
		}
		args = append(args, moreArgs...)
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = bytes.NewBuffer(message)
	cmd.Stderr = logs.NewPrefixer(os.Stderr, "  - ")
	return cmd.Output()
}

func (i *Interceptor) runHTTP(ctx context.Context, message []byte) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.Argument, bytes.NewBuffer(message))
	if err != nil {
		return nil, fmt.Errorf("preparing HTTP request: %w", err)
	}

	client := &http.Client{
		Transport: http.DefaultTransport,
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("making HTTP request: %w", err)
	}
	defer response.Body.Close()

	buf, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("reading HTTP response: %w", err)
	}

	return buf, nil
}

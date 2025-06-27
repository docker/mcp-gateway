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
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/logs"
)

func Callbacks(logCalls, blockSecrets bool, interceptors []Interceptor) server.ToolHandlerMiddleware {
	return func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
		for i := len(interceptors) - 1; i >= 0; i-- {
			next = interceptors[i].Run(next)
		}

		if logCalls {
			next = LogCalls(next)
		}

		if blockSecrets {
			next = BlockSecrets(next)
		}

		return next
	}
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

func (i *Interceptor) Run(next server.ToolHandlerFunc) server.ToolHandlerFunc {
	return func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if i.When == "before" {
			message, err := json.Marshal(request)
			if err != nil {
				return nil, fmt.Errorf("marshalling request: %w", err)
			}

			out, err := i.run(ctx, message)
			if err != nil {
				return nil, fmt.Errorf("executing interceptor: %w", err)
			}

			if len(out) > 0 {
				var response mcp.CallToolResult
				if err := json.Unmarshal(out, &response); err != nil {
					return nil, fmt.Errorf("unmarshalling response: %w", err)
				}

				// If the interceptor returns a response, we use it instead of calling the next handler.
				return &response, nil
			}
		}

		response, err := next(ctx, request)

		if i.When == "after" {
			message, err := json.Marshal(response)
			if err != nil {
				return nil, fmt.Errorf("marshalling response: %w", err)
			}

			_, err = i.run(ctx, message)
			if err != nil {
				return nil, fmt.Errorf("executing interceptor: %w", err)
			}
		}

		return response, err
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

	response, err := http.DefaultClient.Do(request)
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

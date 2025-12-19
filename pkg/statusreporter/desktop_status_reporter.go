package statusreporter

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/google/uuid"
)

type DesktopStatusReporter struct {
	sessionId string
	stateChan chan desktop.McpGatewayState
	eventChan chan desktop.McpGatewayEventsInner
	wg        sync.WaitGroup
	cancel    context.CancelFunc
	running   bool
}

func NewDesktop() *DesktopStatusReporter {
	return &DesktopStatusReporter{
		sessionId: uuid.NewString(),
		stateChan: make(chan desktop.McpGatewayState, 10),
		eventChan: make(chan desktop.McpGatewayEventsInner, 100),
	}
}

func (r *DesktopStatusReporter) LogWriter() io.Writer {
	return &logWriter{r: r}
}

func (r *DesktopStatusReporter) Start(ctx context.Context) {
	if r.running {
		return
	}

	// One for each goroutine
	r.wg.Add(2)

	ctx, r.cancel = context.WithCancel(ctx)
	r.running = true

	go func() {
		defer r.wg.Done()
		defer func() {
			ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cancel()
			if err := desktop.UpdateMcpGatewayState(ctx, r.sessionId, desktop.McpGatewayState{
				Status:    "stopped",
				SessionId: r.sessionId,
			}); err != nil {
				fmt.Fprintf(os.Stderr, "failed to update mcp gateway state: %v\n", err)
			}
		}()
		for {
			select {
			case state := <-r.stateChan:
				state.SessionId = r.sessionId
				if err := desktop.UpdateMcpGatewayState(ctx, state.SessionId, state); err != nil {
					fmt.Fprintf(os.Stderr, "failed to update mcp gateway state: %v\n", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	go func() {
		defer r.wg.Done()

		tick := time.Tick(500 * time.Millisecond)
		var buffer []desktop.McpGatewayEventsInner

		for {
			select {
			case event := <-r.eventChan:
				buffer = append(buffer, event)
			case <-tick:
				if len(buffer) > 0 {
					if err := desktop.PostMcpGatewayEvents(ctx, r.sessionId, buffer); err != nil {
						fmt.Fprintf(os.Stderr, "failed to post mcp gateway events: %v\n", err)
					}
				}
				buffer = make([]desktop.McpGatewayEventsInner, 0)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (r *DesktopStatusReporter) Stop() {
	if !r.running {
		return
	}
	r.running = false
	r.cancel()
	r.wg.Wait()
}

func (r *DesktopStatusReporter) ReportStatus(state desktop.McpGatewayState) {
	select {
	case r.stateChan <- state:
	default:
		fmt.Fprintf(os.Stderr, "failed to report status: channel is full\n")
	}
}

func (r *DesktopStatusReporter) LogEvent(message string) {
	event := desktop.McpGatewayEventsInner{
		Data: map[string]any{
			"message": message,
		},
		Timestamp: time.Now().Format(time.RFC3339),
		Type:      "log",
	}

	select {
	case r.eventChan <- event:
	default:
		fmt.Fprintf(os.Stderr, "failed to log event: channel is full\n")
	}
}

type logWriter struct {
	r *DesktopStatusReporter
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	w.r.LogEvent(string(p))
	return len(p), nil
}

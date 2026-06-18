package oauth

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/docker/mcp-gateway/pkg/desktop"
	"github.com/docker/mcp-gateway/pkg/log"
)

// EventType represents the type of OAuth event from Docker Desktop
type EventType string

const (
	EventLoginStart    EventType = "login-start"
	EventCodeReceived  EventType = "code-received"
	EventLoginSuccess  EventType = "login-success"
	EventTokenRefresh  EventType = "token-refresh"
	EventLogoutSuccess EventType = "logout-success"
	EventError         EventType = "error"
)

// Event represents a parsed OAuth notification event
type Event struct {
	Type     EventType
	Provider string
	Message  string
	Error    string
}

// NotificationMonitor subscribes to Docker Desktop's OAuth notification stream
type NotificationMonitor struct {
	url          string
	client       *http.Client
	OnOAuthEvent func(event Event)
}

// NewNotificationMonitor creates a new notification monitor
func NewNotificationMonitor() *NotificationMonitor {
	// Create HTTP client that uses Docker Desktop's backend Unix socket
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := net.Dialer{}
				return dialer.DialContext(ctx, "unix", desktop.Paths().BackendSocket)
			},
		},
	}

	return &NotificationMonitor{
		url:    "http://localhost/notify/notifications/channel/external-oauth",
		client: client,
	}
}

// Start begins monitoring OAuth notifications from Docker Desktop.
//
// This performs a one-time availability check against the OAuth notification
// endpoint before starting the monitor. If the endpoint or Docker Desktop
// backend socket is unavailable (common on non–Docker Desktop environments),
// the monitor is disabled to avoid repeated connection attempts and log spam.
//
// This ensures the gateway behaves correctly when OAuth notifications are
// not configured or supported, without impacting Docker Desktop users.
func (m *NotificationMonitor) Start(ctx context.Context) {
	// Try once to detect availability
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		log.Logf("- OAuth notification monitor disabled: %v", err)
		return
	}

	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := m.client.Do(req)
	if err != nil {
		log.Logf("- OAuth notification endpoint not available, monitor disabled: %v", err)
		return
	}
	resp.Body.Close()

	// If reachable, start monitor normally
	go m.monitor(ctx)
}

// monitor connects once to the OAuth notification stream.
// If the connection fails or is lost, it does not retry.
func (m *NotificationMonitor) monitor(ctx context.Context) {
	m.connect(ctx)
}

// connect establishes SSE connection and processes events
func (m *NotificationMonitor) connect(ctx context.Context) {
	log.Logf("- Connecting to OAuth notification stream at %s", m.url)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		log.Logf("- Failed to create OAuth notification request: %v", err)
		return
	}

	// Set SSE headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := m.client.Do(req)
	if err != nil {
		log.Logf("- Failed to connect to OAuth notifications: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Logf("- OAuth notification stream unexpected status: %d %s", resp.StatusCode, resp.Status)
		return
	}

	log.Log("- OAuth notification stream connected")

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			log.Log("- OAuth notification stream stopping")
			return
		default:
		}

		line := scanner.Text()

		// Skip empty lines and SSE comments
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}

		// Docker Desktop only sends data: fields
		if jsonData, found := strings.CutPrefix(line, "data: "); found {
			// Skip empty data
			if jsonData == "" {
				continue
			}

			oauthEvent, err := parseOAuthEvent(jsonData)
			if err != nil {
				log.Logf("- Failed to parse OAuth event: %v", err)
				continue
			}

			m.processOAuthEvent(oauthEvent)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Logf("- OAuth notification connection error: %v", err)
	} else {
		log.Log("- OAuth notification stream closed")
	}
}

// parseOAuthEvent parses JSON data from Docker Desktop into an Event
func parseOAuthEvent(jsonData string) (Event, error) {
	var rawEvent map[string]any
	if err := json.Unmarshal([]byte(jsonData), &rawEvent); err != nil {
		return Event{}, fmt.Errorf("failed to parse JSON: %v", err)
	}

	// Extract operation field (Docker Desktop uses 'operation', not 'event_type')
	operation, ok := rawEvent["operation"].(string)
	if !ok {
		return Event{}, fmt.Errorf("missing operation field")
	}

	// Remove mcp-oauth- prefix if present
	eventTypeStr := strings.TrimPrefix(operation, "mcp-oauth-")
	eventType := EventType(eventTypeStr)

	// Extract message and provider
	var message string
	if msg, ok := rawEvent["message"].(string); ok {
		message = msg
	}

	provider := extractProviderFromMessage(message)

	// Extract error field if present
	var errorMsg string
	if err, ok := rawEvent["error"].(string); ok {
		errorMsg = err
	}

	return Event{
		Type:     eventType,
		Provider: provider,
		Message:  message,
		Error:    errorMsg,
	}, nil
}

// processOAuthEvent calls callback for relevant events and logs errors
func (m *NotificationMonitor) processOAuthEvent(event Event) {
	log.Logf("- SSE event received: %s for %s", event.Type, event.Provider)

	// Only log errors and unknown events - let handleOAuthEvent handle success logs
	switch event.Type {
	case EventLoginStart, EventCodeReceived:
		// These are informational events, no callback needed
	case EventLoginSuccess, EventTokenRefresh, EventLogoutSuccess:
		// Trigger callback - handler will log details
		if m.OnOAuthEvent != nil {
			m.OnOAuthEvent(event)
		}
	case EventError:
		if event.Error != "" {
			log.Logf("- OAuth error for %s: %s", event.Provider, event.Error)
		} else {
			log.Logf("- OAuth error for %s (no details)", event.Provider)
		}
	default:
		log.Logf("- Unknown OAuth event type: %s for %s", event.Type, event.Provider)
	}
}

// extractProviderFromMessage extracts provider name from notification message
// Examples:
//
//	"Login successful for linear-remote" -> "linear-remote"
//	"Successfully logged out of github" -> "github"
func extractProviderFromMessage(message string) string {
	// Handle "... for {provider}"
	if strings.Contains(message, " for ") {
		parts := strings.Split(message, " for ")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// Handle "... of {provider}"
	if strings.Contains(message, " of ") {
		parts := strings.Split(message, " of ")
		if len(parts) >= 2 {
			return strings.TrimSpace(parts[len(parts)-1])
		}
	}

	// If no pattern matches, return empty string
	return ""
}

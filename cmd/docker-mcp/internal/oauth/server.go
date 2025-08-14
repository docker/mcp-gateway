package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
	
	"github.com/docker/mcp-gateway/cmd/docker-mcp/internal/desktop"
)

var (
	server      *http.Server
	authCodeChan chan string
	serverMutex  sync.Mutex
	serverWG     sync.WaitGroup
	
	// Store the original state for Docker Desktop exchange
	originalState string
	stateMutex    sync.Mutex
)

// StartCallbackServer starts an HTTP server to receive OAuth callbacks on the specified port
func StartCallbackServer(port int) {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	
	// If server is already running, return
	if server != nil {
		return
	}
	
	authCodeChan = make(chan string, 1)
	
	mux := http.NewServeMux()
	mux.HandleFunc("/", handleCallback) // Handle any path since GitHub might redirect to /
	mux.HandleFunc("/oauth/callback", handleCallback)
	mux.HandleFunc("/callback", handleCallback)
	
	server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	
	serverWG.Add(1)
	go func() {
		defer serverWG.Done()
		fmt.Printf("Starting OAuth callback server on port %d\n", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("OAuth callback server error: %v\n", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	fmt.Printf("Received OAuth callback: %s\n", r.URL.String())
	
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	error := r.URL.Query().Get("error")
	errorDesc := r.URL.Query().Get("error_description")
	
	if error != "" {
		errMsg := fmt.Sprintf("OAuth error: %s - %s", error, errorDesc)
		fmt.Println(errMsg)
		http.Error(w, errMsg, http.StatusBadRequest)
		
		// Send error through channel
		select {
		case authCodeChan <- "":
		default:
		}
		return
	}
	
	if code == "" {
		http.Error(w, "Missing authorization code", http.StatusBadRequest)
		return
	}
	
	fmt.Printf("Received auth code: %s (state: %s)\n", code, state)
	
	// Call Docker Desktop's exchange endpoint to complete OAuth flow
	err := callDockerDesktopExchange(code)
	if err != nil {
		fmt.Printf("Failed to exchange code via Docker Desktop: %v\n", err)
		http.Error(w, "Failed to complete OAuth flow", http.StatusInternalServerError)
		return
	}
	
	fmt.Println("Successfully completed OAuth flow via Docker Desktop")
	
	// Send code to waiting channel
	select {
	case authCodeChan <- code:
		// Success HTML response
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprintf(w, `
<!DOCTYPE html>
<html>
<head>
    <title>Authorization Successful</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            display: flex;
            justify-content: center;
            align-items: center;
            height: 100vh;
            margin: 0;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .container {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0, 0, 0, 0.1);
            text-align: center;
        }
        h1 {
            color: #10b981;
            margin-bottom: 1rem;
        }
        p {
            color: #6b7280;
        }
    </style>
</head>
<body>
    <div class="container">
        <h1>✓ Authorization Successful!</h1>
        <p>You can close this window and return to your application.</p>
    </div>
</body>
</html>
`)
	default:
		http.Error(w, "OAuth flow already completed", http.StatusConflict)
	}
	
	// Schedule server shutdown after handling callback
	go func() {
		time.Sleep(2 * time.Second)
		StopCallbackServer()
	}()
}

// WaitForAuthCode waits for an authorization code from the OAuth callback
func WaitForAuthCode(timeout time.Duration) (string, error) {
	select {
	case code := <-authCodeChan:
		if code == "" {
			return "", fmt.Errorf("OAuth authorization failed")
		}
		return code, nil
	case <-time.After(timeout):
		return "", fmt.Errorf("OAuth callback timeout after %v", timeout)
	}
}

// StopCallbackServer stops the OAuth callback server
func StopCallbackServer() {
	serverMutex.Lock()
	defer serverMutex.Unlock()
	
	if server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if err := server.Shutdown(ctx); err != nil {
			fmt.Printf("Error shutting down OAuth callback server: %v\n", err)
		}
		server = nil
		
		// Wait for server goroutine to finish
		serverWG.Wait()
	}
}

// SetOriginalState stores the original OAuth state for later use with Docker Desktop
func SetOriginalState(state string) {
	stateMutex.Lock()
	defer stateMutex.Unlock()
	originalState = state
}

// callDockerDesktopExchange calls Docker Desktop's /external-oauth2/exchange endpoint
func callDockerDesktopExchange(code string) error {
	stateMutex.Lock()
	state := originalState
	stateMutex.Unlock()
	
	if state == "" {
		return fmt.Errorf("no original state available for exchange")
	}
	
	// Create the request body
	requestBody := map[string]string{
		"code":  code,
		"state": state,
	}
	
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}
	
	// Create a backend client to call Docker Desktop
	client := desktop.ClientBackend
	
	// Call the exchange endpoint
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	err = client.Post(ctx, "/external-oauth2/exchange", bytes.NewReader(jsonBody), nil)
	if err != nil {
		return fmt.Errorf("calling Docker Desktop exchange endpoint: %w", err)
	}
	
	return nil
}
package oauth

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/docker/mcp-gateway/pkg/log"
)

// SDKHandler implements auth.OAuthHandler from the go-sdk, bridging the
// gateway's existing credential storage (CE/Desktop/Community modes) to the
// SDK's transport-level OAuth. This enables automatic Bearer token injection
// and 401/403 retry on StreamableClientTransport.
type SDKHandler struct {
	serverName string
	mode       Mode
	credHelper *CredentialHelper
}

// NewSDKHandler creates a handler for the given server using the specified
// credential storage mode.
func NewSDKHandler(serverName string, mode Mode) *SDKHandler {
	return &SDKHandler{
		serverName: serverName,
		mode:       mode,
		credHelper: NewOAuthCredentialHelperWithMode(mode),
	}
}

// TokenSource returns a token source that reads from the gateway's credential
// store on each Token() call. Returns nil (not an error) when no token exists,
// which tells the transport to skip the Authorization header.
func (h *SDKHandler) TokenSource(ctx context.Context) (oauth2.TokenSource, error) {
	token, err := h.credHelper.GetOAuthToken(ctx, h.serverName)
	if err != nil || token == "" {
		return nil, nil //nolint:nilnil // nil token source means "no auth header"
	}
	return &gatewayTokenSource{
		ctx:        ctx,
		serverName: h.serverName,
		credHelper: h.credHelper,
	}, nil
}

// Authorize is called by the SDK transport on 401/403 responses. The gateway
// runs as a background daemon and cannot interactively open a browser, so this
// method returns an error instructing the user to authorize manually.
// The response body is consumed and closed as required by the interface contract.
func (h *SDKHandler) Authorize(_ context.Context, _ *http.Request, resp *http.Response) error {
	statusCode := 0
	if resp != nil {
		statusCode = resp.StatusCode
		if resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
	}
	log.Logf("! OAuth authorization required for %s (received %d)", h.serverName, statusCode)
	return fmt.Errorf("OAuth authorization required for %s: run 'docker mcp oauth authorize %s' to authenticate", h.serverName, h.serverName)
}

// gatewayTokenSource implements oauth2.TokenSource by reading the current
// access token from the gateway's credential store. Each call to Token()
// fetches the latest token, so background refreshes are picked up automatically.
type gatewayTokenSource struct {
	ctx        context.Context //nolint:containedctx // oauth2.TokenSource.Token() has no context param; storing ctx is the only option
	serverName string
	credHelper *CredentialHelper
}

func (ts *gatewayTokenSource) Token() (*oauth2.Token, error) {
	accessToken, err := ts.credHelper.GetOAuthToken(ts.ctx, ts.serverName)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth token for %s: %w", ts.serverName, err)
	}
	// Expiry is intentionally unset: the SDK calls TokenSource() on every
	// request (no caching), so each call re-reads from the credential store.
	// Background refresh is handled by the gateway's Provider refresh loop,
	// not by the oauth2 library's built-in expiry logic.
	return &oauth2.Token{
		AccessToken: accessToken,
		TokenType:   "Bearer",
	}, nil
}

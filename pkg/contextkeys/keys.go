package contextkeys

// contextKey is a typed key for context values to avoid conflicts
type contextKey string

// OAuthInterceptorEnabledKey is the context key for passing OAuth interceptor feature flag state
const OAuthInterceptorEnabledKey contextKey = "oauthInterceptorEnabled"

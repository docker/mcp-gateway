package remoteurl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"time"
)

// AllowInsecureRemoteURLEnv enables local/dev remote MCP endpoints. Production
// defaults allow only public HTTPS destinations.
const AllowInsecureRemoteURLEnv = "DOCKER_MCP_ALLOW_INSECURE_REMOTE_URLS"

type resolver interface {
	LookupNetIP(ctx context.Context, network, host string) ([]netip.Addr, error)
}

type contextKey struct{}

type Options struct {
	AllowInsecure bool
	Resolver      resolver
}

type Validator struct {
	allowInsecure bool
	resolver      resolver
}

func NewValidator(options Options) Validator {
	resolver := options.Resolver
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return Validator{
		allowInsecure: options.AllowInsecure,
		resolver:      resolver,
	}
}

func DefaultValidator() Validator {
	return NewValidator(Options{
		AllowInsecure: allowInsecureFromEnv(),
		Resolver:      net.DefaultResolver,
	})
}

func Validate(ctx context.Context, rawURL string) error {
	return DefaultValidator().Validate(ctx, rawURL)
}

// UpgradeKnownHTTPURLToHTTPS rewrites legacy public HTTP URLs that are known to
// serve the same content over HTTPS. It deliberately does not relax validation
// for arbitrary HTTP URLs.
func UpgradeKnownHTTPURLToHTTPS(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	if !strings.EqualFold(u.Scheme, "http") {
		return rawURL
	}
	if !strings.EqualFold(u.Hostname(), "desktop.docker.com") {
		return rawURL
	}
	switch u.Port() {
	case "":
		u.Scheme = "https"
	case "80":
		u.Scheme = "https"
		u.Host = u.Hostname()
	default:
		return rawURL
	}
	return u.String()
}

func (v Validator) Validate(ctx context.Context, rawURL string) error {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return fmt.Errorf("remote URL is empty")
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid remote URL: %w", err)
	}
	return v.ValidateURL(ctx, u)
}

func (v Validator) ValidateURL(ctx context.Context, u *url.URL) error {
	host, needsResolution, err := v.validateURLWithoutResolution(u)
	if err != nil {
		return err
	}
	if !needsResolution {
		return nil
	}

	ips, err := v.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return fmt.Errorf("resolving remote URL host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("remote URL host %q did not resolve to any IP addresses", host)
	}
	for _, ip := range ips {
		if err := validateAddr(ip); err != nil {
			return fmt.Errorf("remote URL host %q resolved to disallowed address %s: %w", host, ip, err)
		}
	}

	return nil
}

// ValidateURLWithoutResolution applies URL safety checks that do not require DNS.
// It rejects unsafe schemes, userinfo, unsafe hostname forms, and disallowed IP
// literals. Call ValidateURL or use a guarded transport before network access.
func (v Validator) ValidateURLWithoutResolution(u *url.URL) error {
	_, _, err := v.validateURLWithoutResolution(u)
	return err
}

func (v Validator) validateURLWithoutResolution(u *url.URL) (string, bool, error) {
	if u == nil {
		return "", false, fmt.Errorf("remote URL is empty")
	}
	if u.Scheme == "" || u.Host == "" {
		return "", false, fmt.Errorf("remote URL must be absolute")
	}
	if u.User != nil {
		return "", false, fmt.Errorf("remote URL must not include userinfo")
	}

	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "https":
	case "http":
		if !v.allowInsecure {
			return "", false, fmt.Errorf("remote URL must use https")
		}
	default:
		return "", false, fmt.Errorf("remote URL must use http or https")
	}

	host := u.Hostname()
	if host == "" {
		return "", false, fmt.Errorf("remote URL host is empty")
	}
	if strings.ContainsAny(host, "\x00%") {
		return "", false, fmt.Errorf("remote URL host is malformed")
	}

	if v.allowInsecure {
		return host, false, nil
	}

	if isBlockedHostname(host) {
		return "", false, fmt.Errorf("remote URL host %q is not allowed", host)
	}

	if ip, err := netip.ParseAddr(host); err == nil {
		if err := validateAddr(ip); err != nil {
			return "", false, fmt.Errorf("remote URL host %q is not allowed: %w", host, err)
		}
		return host, false, nil
	}

	return host, true, nil
}

func GuardTransport(base http.RoundTripper) http.RoundTripper {
	return DefaultValidator().GuardTransport(base)
}

func DirectTransport() http.RoundTripper {
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		cloned := transport.Clone()
		cloned.Proxy = nil
		return cloned
	}
	return &http.Transport{}
}

func GuardDirectTransport() http.RoundTripper {
	return DefaultValidator().GuardTransport(DirectTransport())
}

func (v Validator) GuardTransport(base http.RoundTripper) http.RoundTripper {
	return v.guardTransport(base, false)
}

// TrustedProxyDialer dials a trusted local HTTP proxy socket.
type TrustedProxyDialer func(context.Context) (net.Conn, error)

// GuardTrustedProxyDialer validates request and redirect URLs, then sends
// traffic through a trusted local HTTP proxy socket dialer. Use it only with
// Docker Desktop's hardcoded local proxy socket.
//
// Unlike GuardTransport's direct path, this proxy path cannot pin the dialed IP
// after validation because Docker Desktop's proxy resolves and dials the final
// target. The validator still rejects blocked request and redirect URLs before
// the proxy is dialed, but a narrow DNS-rebinding race remains accepted here to
// preserve Docker Desktop proxy semantics.
func GuardTrustedProxyDialer(dialer TrustedProxyDialer) http.RoundTripper {
	return DefaultValidator().GuardTrustedProxyDialer(dialer)
}

// GuardTrustedProxyDialer validates request and redirect URLs, then sends
// traffic through a trusted local HTTP proxy socket dialer. Use it only with
// Docker Desktop's hardcoded local proxy socket.
//
// Unlike GuardTransport's direct path, this proxy path cannot pin the dialed IP
// after validation because Docker Desktop's proxy resolves and dials the final
// target. The validator still rejects blocked request and redirect URLs before
// the proxy is dialed, but a narrow DNS-rebinding race remains accepted here to
// preserve Docker Desktop proxy semantics.
func (v Validator) GuardTrustedProxyDialer(dialer TrustedProxyDialer) http.RoundTripper {
	return v.guardTransport(trustedProxyTransport(dialer), true)
}

func (v Validator) guardTransport(base http.RoundTripper, allowTrustedProxy bool) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	base = v.guardDialer(base, allowTrustedProxy)
	return &validatingRoundTripper{
		base:      base,
		validator: v,
	}
}

func NewHTTPClient(timeout time.Duration, base http.RoundTripper) *http.Client {
	validator := DefaultValidator()
	return &http.Client{
		Timeout:   timeout,
		Transport: validator.GuardTransport(base),
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validator.ValidateURL(req.Context(), req.URL)
		},
	}
}

// NewTrustedProxyHTTPClient returns a guarded client for a trusted local proxy
// dialer, such as Docker Desktop's proxy socket dialer.
func NewTrustedProxyHTTPClient(timeout time.Duration, dialer TrustedProxyDialer) *http.Client {
	validator := DefaultValidator()
	return &http.Client{
		Timeout:   timeout,
		Transport: validator.GuardTrustedProxyDialer(dialer),
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validator.ValidateURL(req.Context(), req.URL)
		},
	}
}

func NewDirectHTTPClient(timeout time.Duration) *http.Client {
	validator := DefaultValidator()
	return &http.Client{
		Timeout:   timeout,
		Transport: validator.GuardTransport(DirectTransport()),
		CheckRedirect: func(req *http.Request, _ []*http.Request) error {
			return validator.ValidateURL(req.Context(), req.URL)
		},
	}
}

func (v Validator) guardDialer(base http.RoundTripper, allowTrustedProxy bool) http.RoundTripper {
	transport, ok := base.(*http.Transport)
	if !ok {
		return base
	}

	cloned := transport.Clone()
	originalDialContext := cloned.DialContext
	if originalDialContext == nil {
		originalDialContext = (&net.Dialer{}).DialContext
	}

	cloned.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return originalDialContext(ctx, network, address)
		}

		targetHost, ok := ctx.Value(contextKey{}).(string)
		if !ok || targetHost == "" {
			return nil, fmt.Errorf("remote URL target host missing from request context")
		}

		if v.allowInsecure {
			return originalDialContext(ctx, network, address)
		}
		if !sameHostname(targetHost, host) {
			if allowTrustedProxy {
				return originalDialContext(ctx, network, address)
			}
			return nil, fmt.Errorf("remote URL dial target %q does not match request host %q", host, targetHost)
		}

		return v.dialAllowedAddress(ctx, originalDialContext, network, host, port)
	}

	return cloned
}

func trustedProxyTransport(dialer TrustedProxyDialer) http.RoundTripper {
	if dialer == nil {
		dialer = func(context.Context) (net.Conn, error) {
			return nil, fmt.Errorf("trusted proxy dialer is nil")
		}
	}
	return &http.Transport{
		Proxy: http.ProxyURL(&url.URL{
			Scheme: "http",
		}),
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dialer(ctx)
		},
	}
}

func (v Validator) dialAllowedAddress(ctx context.Context, dial func(context.Context, string, string) (net.Conn, error), network, host, port string) (net.Conn, error) {
	if ip, err := netip.ParseAddr(host); err == nil {
		if err := validateAddr(ip); err != nil {
			return nil, err
		}
		return dial(ctx, network, net.JoinHostPort(ip.String(), port))
	}

	ips, err := v.resolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%q did not resolve to any IP addresses", host)
	}
	for _, ip := range ips {
		if err := validateAddr(ip); err != nil {
			return nil, fmt.Errorf("%q resolved to disallowed address %s: %w", host, ip, err)
		}
	}

	var lastErr error
	for _, ip := range ips {
		conn, err := dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

type validatingRoundTripper struct {
	base      http.RoundTripper
	validator Validator
}

func (t *validatingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if err := t.validator.ValidateURL(req.Context(), req.URL); err != nil {
		return nil, err
	}

	ctx := context.WithValue(req.Context(), contextKey{}, req.URL.Hostname())
	return t.base.RoundTrip(req.Clone(ctx))
}

func allowInsecureFromEnv() bool {
	return os.Getenv(AllowInsecureRemoteURLEnv) == "1" ||
		strings.EqualFold(os.Getenv(AllowInsecureRemoteURLEnv), "true")
}

func sameHostname(a, b string) bool {
	return normalizeHostname(a) == normalizeHostname(b)
}

func normalizeHostname(host string) string {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	return host
}

func isBlockedHostname(host string) bool {
	host = normalizeHostname(host)
	switch host {
	case "localhost", "metadata", "metadata.google.internal", "metadata.azure.internal":
		return true
	}

	blockedSuffixes := []string{
		".localhost",
		".local",
		".localdomain",
		".internal",
		".cluster.local",
		".svc",
	}
	for _, suffix := range blockedSuffixes {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return false
}

var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("169.254.0.0/16"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("192.0.2.0/24"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("198.18.0.0/15"),
	netip.MustParsePrefix("198.51.100.0/24"),
	netip.MustParsePrefix("203.0.113.0/24"),
	netip.MustParsePrefix("224.0.0.0/4"),
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("255.255.255.255/32"),
	netip.MustParsePrefix("::/128"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("64:ff9b::/96"),
	netip.MustParsePrefix("fc00::/7"),
	netip.MustParsePrefix("fe80::/10"),
	netip.MustParsePrefix("ff00::/8"),
	netip.MustParsePrefix("2001:db8::/32"),
}

func validateAddr(ip netip.Addr) error {
	if !ip.IsValid() {
		return fmt.Errorf("invalid IP address")
	}
	if ip.Zone() != "" {
		return fmt.Errorf("scoped IPv6 addresses are not allowed")
	}

	ip = ip.Unmap()
	for _, prefix := range blockedPrefixes {
		if prefix.Contains(ip) {
			return fmt.Errorf("address is in blocked range %s", prefix)
		}
	}
	if !ip.IsGlobalUnicast() || ip.IsLoopback() || ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() || ip.IsUnspecified() {
		return fmt.Errorf("address is not public")
	}

	return nil
}

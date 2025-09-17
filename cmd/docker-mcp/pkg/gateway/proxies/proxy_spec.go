package proxies

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
)

type Protocol int

func (p Protocol) String() string {
	switch p {
	case HTTP:
		return "http"
	case TCP:
		return "tcp"
	default:
		return "unknown"
	}
}

const (
	HTTP Protocol = iota
	TCP
)

// Proxy represents either a L4 (TCP/UDP), or an L7 (HTTP) proxy for a
// particular hostname:port.
type Proxy struct {
	Protocol Protocol // Protocol is either HTTP or TCP
	Hostname string
	Port     uint16
}

// ParseProxySpec takes a string representing a Proxy spec, and returns a Proxy
// or an error if the spec is invalid. A proxy spec is a string of the form
// "hostname:port/protocol", where:
//
// - hostname is a DNS hostname or an IP address
// - port is a port number
// - protocol is either "http", "https" or "tcp"
func ParseProxySpec(spec string) (Proxy, error) {
	parts := strings.Split(spec, "/")
	if len(parts) > 2 {
		return Proxy{}, fmt.Errorf("invalid proxy spec %q", spec)
	}

	hostname, portStr, err := net.SplitHostPort(parts[0])
	if err != nil {
		return Proxy{}, fmt.Errorf("invalid proxy spec %q: %w", spec, err)
	}
	if portStr == "" {
		return Proxy{}, fmt.Errorf("invalid proxy spec %q: missing port", spec)
	}

	// Consider the hostname component is a DNS hostname if it's not a valid IP
	// address.
	if ip, err := netip.ParseAddr(hostname); err == nil {
		// If it's an IP, disallow localhost, and multicast addresses.
		if ip.IsLoopback() || ip.IsMulticast() {
			return Proxy{}, fmt.Errorf("invalid proxy spec %q: invalid hostname", spec)
		}
	}

	port, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return Proxy{}, fmt.Errorf("invalid proxy spec %q: invalid port", spec)
	}

	protocol := HTTP
	if len(parts) == 2 {
		switch parts[1] {
		case "http":
			protocol = HTTP
		case "https":
			protocol = HTTP
		case "tcp":
			protocol = TCP
		default:
			return Proxy{}, fmt.Errorf("invalid proxy spec %q: invalid protocol", spec)
		}
	}

	return Proxy{
		Protocol: protocol,
		Hostname: hostname,
		Port:     uint16(port),
	}, nil
}

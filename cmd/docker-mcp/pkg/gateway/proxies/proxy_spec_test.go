package proxies

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseProxySpec(t *testing.T) {
	testcases := []struct {
		name     string
		spec     string
		expProxy Proxy
		expErr   string
	}{
		{
			name:     "valid http spec",
			spec:     "localhost:8080/http",
			expProxy: Proxy{Protocol: HTTP, Hostname: "localhost", Port: 8080},
		},
		{
			name:     "valid https spec",
			spec:     "localhost:8080/https",
			expProxy: Proxy{Protocol: HTTP, Hostname: "localhost", Port: 8080},
		},
		{
			name:     "valid tcp spec",
			spec:     "localhost:8080/tcp",
			expProxy: Proxy{Protocol: TCP, Hostname: "localhost", Port: 8080},
		},
		{
			name:   "invalid spec, no port/proto",
			spec:   "foobar",
			expErr: `invalid proxy spec "foobar"`,
		},
		{
			name:   "invalid spec, no port",
			spec:   "foobar/tcp",
			expErr: `invalid proxy spec "foobar/tcp": address foobar: missing port in address`,
		},
		{
			name:   "invalid spec, extra slash",
			spec:   "localhost:8080/tcp/foobar",
			expErr: `invalid proxy spec "localhost:8080/tcp/foobar"`,
		},
		{
			name:   "invalid spec, unspec port",
			spec:   "localhost:/tcp",
			expErr: `invalid proxy spec "localhost:/tcp": missing port`,
		},
		{
			name:   "invalid spec, multicast hostname",
			spec:   "[ff00::1]:80/tcp",
			expErr: `invalid proxy spec "[ff00::1]:80/tcp": invalid hostname`,
		},
		{
			name:   "invalid spec, invalid port",
			spec:   "google.com:100000/tcp",
			expErr: `invalid proxy spec "google.com:100000/tcp": invalid port`,
		},
		{
			name:   "invalid spec, invalid proto",
			spec:   "google.com:443/foobar",
			expErr: `invalid proxy spec "google.com:443/foobar": invalid protocol`,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			proxy, err := ParseProxySpec(tc.spec)
			assert.Equal(t, tc.expProxy, proxy)

			if tc.expErr != "" {
				assert.ErrorContains(t, err, tc.expErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

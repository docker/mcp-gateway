package gateway

import "testing"

func TestServerImplementationName(t *testing.T) {
	g := &Gateway{}
	if got := g.serverImplementation().Name; got != defaultGatewayServerName {
		t.Fatalf("default name = %q", got)
	}
	if got := g.serverImplementation().Version; got != gatewayServerVersion {
		t.Fatalf("version = %q", got)
	}

	g.GatewayName = "  DevTools  "
	if got := g.serverImplementation().Name; got != "DevTools" {
		t.Fatalf("trimmed name = %q", got)
	}

	g.GatewayName = " \t "
	if got := g.serverImplementation().Name; got != defaultGatewayServerName {
		t.Fatalf("blank name = %q", got)
	}
}

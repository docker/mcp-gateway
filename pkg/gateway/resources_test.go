package gateway

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/gateway/proxies"
	"github.com/docker/mcp-gateway/pkg/workingset"
)

func TestServerResourceLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		requested *catalog.Resources
		cpus      map[string]string
		memory    map[string]string
		want      catalog.Resources
		wantErr   string
	}{
		{name: "legacy defaults", want: catalog.Resources{CPUs: "1", Memory: "2Gb"}},
		{name: "empty block", requested: &catalog.Resources{}, want: catalog.Resources{CPUs: "1", Memory: "2Gb"}},
		{name: "smaller server", requested: &catalog.Resources{CPUs: "0.5", Memory: "256m"}, want: catalog.Resources{CPUs: "0.5", Memory: "256m"}},
		{name: "partial catalog", requested: &catalog.Resources{Memory: "512m"}, want: catalog.Resources{CPUs: "1", Memory: "512m"}},
		{name: "equivalent units", requested: &catalog.Resources{CPUs: "1.0", Memory: "2048MiB"}, want: catalog.Resources{CPUs: "1.0", Memory: "2048MiB"}},
		{name: "catalog cannot raise CPU", requested: &catalog.Resources{CPUs: "2"}, wantErr: "--server-cpus worker=2"},
		{name: "catalog cannot raise memory", requested: &catalog.Resources{Memory: "4g"}, wantErr: "--server-memory worker=4g"},
		{name: "explicit increase", requested: &catalog.Resources{CPUs: "2", Memory: "4g"}, cpus: map[string]string{"worker": "2"}, memory: map[string]string{"worker": "4g"}, want: catalog.Resources{CPUs: "2", Memory: "4g"}},
		{name: "operator wins", requested: &catalog.Resources{CPUs: "8", Memory: "8g"}, cpus: map[string]string{"worker": "0.25"}, memory: map[string]string{"worker": "128m"}, want: catalog.Resources{CPUs: "0.25", Memory: "128m"}},
		{name: "override without catalog block", cpus: map[string]string{"worker": "2.5"}, want: catalog.Resources{CPUs: "2.5", Memory: "2Gb"}},
		{name: "override is scoped", cpus: map[string]string{"other": "8"}, requested: &catalog.Resources{CPUs: "2"}, wantErr: "exceeds global"},
		{name: "fields independent", cpus: map[string]string{"worker": "2"}, requested: &catalog.Resources{Memory: "4g"}, wantErr: "--server-memory"},
		{name: "cannot disable catalog CPU limit", requested: &catalog.Resources{CPUs: "0"}, wantErr: "resources.cpus"},
		{name: "cannot disable catalog memory limit", requested: &catalog.Resources{Memory: "0"}, wantErr: "resources.memory"},
		{name: "empty override rejected", cpus: map[string]string{"worker": ""}, wantErr: "--server-cpus"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := &clientPool{Options: Options{Cpus: 1, Memory: "2Gb", ServerCPUs: tc.cpus, ServerMemory: tc.memory}}
			server := &catalog.ServerConfig{Name: "worker", Spec: catalog.Server{Image: "example/worker", Resources: tc.requested}}
			args, _, err := cp.argsAndEnv(server, proxies.TargetConfig{})
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
				assert.Nil(t, args)
				return
			}
			require.NoError(t, err)
			// Exercise the actual container argument path, not just the resolver.
			assert.Equal(t, 1, countArg(args, "--cpus"))
			assert.Equal(t, 1, countArg(args, "--memory"))
			assert.Equal(t, tc.want.CPUs, args[7])
			assert.Equal(t, tc.want.Memory, args[9])
		})
	}
}

func countArg(args []string, target string) int {
	n := 0
	for _, arg := range args {
		if arg == target {
			n++
		}
	}
	return n
}

func TestResourceLimitValidation(t *testing.T) {
	for _, value := range []string{"", "0", "-1", "NaN", "Inf", "1/2", "1e3", " 1", "1 ", "1.2.3", "0.001", "0.1234567891", "9223372037", "18446744074"} {
		t.Run("cpu="+value, func(t *testing.T) {
			o := Options{ServerCPUs: map[string]string{"worker": value}}
			require.ErrorContains(t, o.ValidateResourceOverrides(), "--server-cpus")
		})
	}
	for _, value := range []string{"", "0", "-1", "5m", "NaN", "Inf", "garbage", "999999999999999999999999999999g"} {
		t.Run("memory="+value, func(t *testing.T) {
			o := Options{ServerMemory: map[string]string{"worker": value}}
			require.ErrorContains(t, o.ValidateResourceOverrides(), "--server-memory")
		})
	}
	for _, name := range []string{"", " worker", "worker "} {
		o := Options{ServerCPUs: map[string]string{name: "1"}}
		require.ErrorContains(t, o.ValidateResourceOverrides(), "server name")
	}
	for _, value := range []string{"0.01", "0.123456789", "1", "2.5"} {
		o := Options{ServerCPUs: map[string]string{"worker": value}, ServerMemory: map[string]string{"worker": "6MiB"}}
		require.NoError(t, o.ValidateResourceOverrides())
	}
}

func TestServerResourcesWithUnlimitedGlobals(t *testing.T) {
	server := &catalog.ServerConfig{Name: "worker", Spec: catalog.Server{Resources: &catalog.Resources{CPUs: "2", Memory: "4g"}}}
	for _, memory := range []string{"", "0"} {
		o := Options{Memory: memory}
		got, err := o.serverResources(server)
		require.NoError(t, err)
		assert.Equal(t, *server.Spec.Resources, got)
	}
}

func TestInvalidResourcesFailBeforeDockerAccess(t *testing.T) {
	server := catalog.Server{Image: "example/worker", Resources: &catalog.Resources{CPUs: "2"}}
	g := &Gateway{Options: Options{Cpus: 1, Memory: "2g"}}
	// No Docker client is provided: validation must happen before pull/inspect.
	err := g.pullAndVerify(context.Background(), Configuration{serverNames: []string{"worker"}, servers: map[string]catalog.Server{"worker": server}})
	require.ErrorContains(t, err, "--server-cpus worker=2")
	cp := &clientPool{Options: g.Options}
	// Dynamic acquisition must reject the same request before creating a client.
	_, err = cp.AcquireClient(context.Background(), &catalog.ServerConfig{Name: "worker", Spec: server}, nil)
	require.ErrorContains(t, err, "--server-cpus worker=2")
}

func TestResourceLimitsDoNotApplyToExternalServers(t *testing.T) {
	o := Options{Cpus: 1, Memory: "2g"}
	server := &catalog.ServerConfig{Name: "remote", Spec: catalog.Server{Remote: catalog.Remote{URL: "https://example.com/mcp"}, Resources: &catalog.Resources{CPUs: "invalid"}}}
	require.NoError(t, o.validateServerResources(server))
	server.Spec.Remote.URL = ""
	server.Spec.Image = "example/worker"
	o.Static = true
	require.NoError(t, o.validateServerResources(server))
}

func TestCatalogResourceRoundTrip(t *testing.T) {
	// Profiles and OCI catalog snapshots embed catalog.Server and use these codecs.
	var server catalog.Server
	require.NoError(t, yaml.Unmarshal([]byte("name: worker\ntype: server\nimage: example/worker\nresources:\n  cpus: '0.5'\n  memory: 256m\n"), &server))
	require.NotNil(t, server.Resources)
	for _, codec := range []struct {
		name      string
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{{"json", json.Marshal, json.Unmarshal}, {"yaml", yaml.Marshal, yaml.Unmarshal}} {
		t.Run(codec.name, func(t *testing.T) {
			data, err := codec.marshal(server)
			require.NoError(t, err)
			var got catalog.Server
			require.NoError(t, codec.unmarshal(data, &got))
			assert.Equal(t, server.Resources, got.Resources)
			data, err = codec.marshal(catalog.Server{Name: "legacy"})
			require.NoError(t, err)
			assert.NotContains(t, string(data), "resources")
		})
	}
}

func TestDynamicAddRejectsResourcesWithoutMutatingConfiguration(t *testing.T) {
	g := &Gateway{
		Options:      Options{Cpus: 1, Memory: "2g"},
		policyClient: newMockPolicyClient(),
		configuration: Configuration{
			serverNames: []string{"existing"},
			servers: map[string]catalog.Server{"worker": {
				Image:     "example/worker",
				Resources: &catalog.Resources{CPUs: "2"},
			}},
		},
	}
	result, err := addServerHandler(g, nil)(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Name: "mcp-add", Arguments: json.RawMessage(`{"name":"worker"}`)},
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError)
	require.Len(t, result.Content, 1)
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "--server-cpus worker=2")
	assert.Equal(t, []string{"existing"}, g.configuration.serverNames)
}

func TestProfileRejectsResourcesBeforeAnyImagePull(t *testing.T) {
	g := &Gateway{Options: Options{Cpus: 1, Memory: "2g"}, policyClient: newMockPolicyClient()}
	ws := workingset.WorkingSet{Name: "development", Servers: []workingset.Server{
		{Type: workingset.ServerTypeImage, Snapshot: &workingset.ServerSnapshot{Server: catalog.Server{Name: "light", Image: "example/light"}}},
		{Type: workingset.ServerTypeImage, Snapshot: &workingset.ServerSnapshot{Server: catalog.Server{Name: "heavy", Image: "example/heavy", Resources: &catalog.Resources{CPUs: "2"}}}},
	}}
	// Even the valid first server must not reach the nil Docker client.
	require.ErrorContains(t, g.ActivateProfile(context.Background(), ws), "--server-cpus heavy=2")
	assert.Empty(t, g.configuration.serverNames)
}

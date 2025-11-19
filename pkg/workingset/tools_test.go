package workingset

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/docker/mcp-gateway/pkg/catalog"
	"github.com/docker/mcp-gateway/pkg/db"
)

func makeServer(name string, snapshotTools []catalog.Tool, initialTools ...[]string) db.Server {
	var tools []string
	if len(initialTools) > 0 {
		tools = initialTools[0]
	}

	return db.Server{
		Type:  "image",
		Image: "test:latest",
		Tools: tools,
		Snapshot: &db.ServerSnapshot{
			Server: catalog.Server{
				Name:  name,
				Tools: snapshotTools,
			},
		},
	}
}

func TestToolsUpdateOperations(t *testing.T) {
	tests := []struct {
		name           string
		servers        []db.Server
		enableArgs     []string
		disableArgs    []string
		enableAllArgs  []string
		disableAllArgs []string
		expectedOutput string
		makeAssertions func(t *testing.T, server db.ServerList)
	}{
		{
			name: "tools uninitialized, enable one tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}),
			},
			enableArgs:     []string{"test-server.test-tool-1"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				t.Helper()
				assert.Len(t, servers[0].Tools, 1)
				assert.Equal(t, "test-tool-1", servers[0].Tools[0])
			},
		},
		{
			name: "tools uninitialized, disable one tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "tools uninitialized, enable all tools",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"test-server"},
			disableAllArgs: []string{},
			expectedOutput: "No changes made to profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
			},
		},
		{
			name: "tools uninitialized, disable all tools",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"test-server"},
			expectedOutput: "Disabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "tools uninitialized, disable one tool, from many tools",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}, {Name: "test-tool-3"}}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.NotContains(t, servers[0].Tools, "test-tool-1")
				assert.Contains(t, servers[0].Tools, "test-tool-2")
				assert.Contains(t, servers[0].Tools, "test-tool-3")
				assert.Len(t, servers[0].Tools, 2)
			},
		},
		{
			name: "one tool, enable same tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{"test-server.test-tool-1"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "No changes made to profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
			},
		},
		{
			name: "one tool, enable new tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{"test-server.test-tool-2"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
				assert.Contains(t, servers[0].Tools, "test-tool-2")
			},
		},
		{
			name: "one tool, disable same tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "one tool, disable non-existent tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-2"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "No changes made to profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "one tool, enable all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"test-server"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
			},
		},
		{
			name: "one tool, disable all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"test-server"},
			expectedOutput: "Disabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.NotNil(t, servers[0].Tools)
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "empty tools, enable one tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{}),
			},
			enableArgs:     []string{"test-server.test-tool-1"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "empty tools, disable one tool (no-op)",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "No changes made to profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "empty tools, enable all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"test-server"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
			},
		},
		{
			name: "empty tools, disable all (no-op)",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"test-server"},
			expectedOutput: "No changes made to profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "multiple tools, enable one more",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}, {Name: "test-tool-3"}}, []string{"test-tool-1", "test-tool-2"}),
			},
			enableArgs:     []string{"test-server.test-tool-3"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
				assert.Contains(t, servers[0].Tools, "test-tool-2")
				assert.Contains(t, servers[0].Tools, "test-tool-3")
				assert.Len(t, servers[0].Tools, 3)
			},
		},
		{
			name: "multiple tools, disable one",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}, {Name: "test-tool-3"}}, []string{"test-tool-1", "test-tool-2"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.NotContains(t, servers[0].Tools, "test-tool-1")
				assert.Contains(t, servers[0].Tools, "test-tool-2")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "multiple tools, enable all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}, {Name: "test-tool-3"}}, []string{"test-tool-1", "test-tool-2"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"test-server"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
			},
		},
		{
			name: "multiple tools, disable all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}, {Name: "test-tool-3"}}, []string{"test-tool-1", "test-tool-2"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"test-server"},
			expectedOutput: "Disabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.NotNil(t, servers[0].Tools)
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "multiple servers, enable tool on each",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{}),
			},
			enableArgs:     []string{"server-1.tool-1", "server-2.tool-2"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 2 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "tool-1")
				assert.Contains(t, servers[1].Tools, "tool-2")
			},
		},
		{
			name: "multiple servers, enable all on one server only",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"server-1"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
				assert.NotNil(t, servers[1].Tools)
				assert.Empty(t, servers[1].Tools)
			},
		},
		{
			name: "enable and disable same tool (conflict)",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{}),
			},
			enableArgs:     []string{"test-server.test-tool-1"},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 1 tool(s) disabled\nWarning: The following tool(s) were both enabled and disabled in the same operation: test-server.test-tool-1\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				// Should be empty since disable happens after enable
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "enable-all on server1, disable-all on server2",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{"tool-1"}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{"tool-2"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"server-1"},
			disableAllArgs: []string{"server-2"},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\nDisabled all tools for 1 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
				assert.NotNil(t, servers[1].Tools)
				assert.Empty(t, servers[1].Tools)
			},
		},
		{
			name: "enable-all on server1, enable specific tool on server2",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{"tool-1"}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{}),
			},
			enableArgs:     []string{"server-2.tool-2"},
			disableArgs:    []string{},
			enableAllArgs:  []string{"server-1"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\nUpdated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
				assert.Contains(t, servers[1].Tools, "tool-2")
			},
		},
		{
			name: "enable-all on one server, then disable specific tool on same server",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1"},
			enableAllArgs:  []string{"test-server"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\nUpdated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				// Enable-all sets to nil, then disable expands and removes tool-1
				assert.Contains(t, servers[0].Tools, "test-tool-2")
				assert.NotContains(t, servers[0].Tools, "test-tool-1")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "disable-all on one server, then enable specific tool on same server",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{"test-server.test-tool-2"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"test-server"},
			expectedOutput: "Disabled all tools for 1 server(s) in profile test-set\nUpdated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				// Disable-all sets to empty, then enable adds tool-2
				assert.Contains(t, servers[0].Tools, "test-tool-2")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "enable same tool twice",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{}),
			},
			enableArgs:     []string{"test-server.test-tool-1", "test-server.test-tool-1"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 1 tool(s) enabled, 0 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "test-tool-1")
				assert.Len(t, servers[0].Tools, 1)
			},
		},
		{
			name: "disable same tool twice",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"test-server.test-tool-1", "test-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 0 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "multiple conflicts",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}, {Name: "test-tool-2"}}, []string{}),
			},
			enableArgs:     []string{"test-server.test-tool-1", "test-server.test-tool-2"},
			disableArgs:    []string{"test-server.test-tool-1", "test-server.test-tool-2"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedOutput: "Updated profile test-set: 2 tool(s) enabled, 2 tool(s) disabled\nWarning: The following tool(s) were both enabled and disabled in the same operation: test-server.test-tool-1, test-server.test-tool-2\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Empty(t, servers[0].Tools)
			},
		},
		{
			name: "enable-all on multiple servers",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{"tool-1"}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{"tool-2"}),
				makeServer("server-3", []catalog.Tool{{Name: "tool-3"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"server-1", "server-2", "server-3"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 3 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Nil(t, servers[0].Tools)
				assert.Nil(t, servers[1].Tools)
				assert.Nil(t, servers[2].Tools)
			},
		},
		{
			name: "disable-all on multiple servers",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}}, []string{"tool-1"}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-2"}}, []string{"tool-2"}),
				makeServer("server-3", []catalog.Tool{{Name: "tool-3"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"server-1", "server-2", "server-3"},
			expectedOutput: "Disabled all tools for 2 server(s) in profile test-set\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.NotNil(t, servers[0].Tools)
				assert.Empty(t, servers[0].Tools)
				assert.NotNil(t, servers[1].Tools)
				assert.Empty(t, servers[1].Tools)
				assert.Empty(t, servers[2].Tools)
			},
		},
		{
			name: "both enable-all and individual enable/disable in same operation",
			servers: []db.Server{
				makeServer("server-1", []catalog.Tool{{Name: "tool-1"}, {Name: "tool-2"}}, []string{"tool-1"}),
				makeServer("server-2", []catalog.Tool{{Name: "tool-3"}, {Name: "tool-4"}}, []string{"tool-3"}),
			},
			enableArgs:     []string{"server-2.tool-4"},
			disableArgs:    []string{"server-1.tool-1"},
			enableAllArgs:  []string{"server-1"},
			disableAllArgs: []string{},
			expectedOutput: "Enabled all tools for 1 server(s) in profile test-set\nUpdated profile test-set: 1 tool(s) enabled, 1 tool(s) disabled\n",
			makeAssertions: func(t *testing.T, servers db.ServerList) {
				t.Helper()
				assert.Contains(t, servers[0].Tools, "tool-2")
				assert.NotContains(t, servers[0].Tools, "tool-1")
				assert.Contains(t, servers[1].Tools, "tool-3")
				assert.Contains(t, servers[1].Tools, "tool-4")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			err := dao.CreateWorkingSet(ctx, db.WorkingSet{
				ID:      "test-set",
				Name:    "Test Working Set",
				Servers: tt.servers,
				Secrets: db.SecretMap{},
			})
			require.NoError(t, err)

			output := captureStdout(func() {
				err = UpdateTools(ctx, dao, "test-set", tt.enableArgs, tt.disableArgs, tt.enableAllArgs, tt.disableAllArgs)
				require.NoError(t, err)
			})

			if tt.expectedOutput != "" {
				assert.Equal(t, tt.expectedOutput, output)
			}

			if tt.makeAssertions != nil {
				dbSet, err := dao.GetWorkingSet(ctx, "test-set")
				require.NoError(t, err)
				tt.makeAssertions(t, dbSet.Servers)
			}
		})
	}
}

func TestToolsUpdateOperationsErrors(t *testing.T) {
	tests := []struct {
		name           string
		servers        []db.Server
		enableArgs     []string
		disableArgs    []string
		enableAllArgs  []string
		disableAllArgs []string
		expectedError  string
	}{
		{
			name: "server not found when enabling a tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{}),
			},
			enableArgs:     []string{"nonexistent-server.test-tool-1"},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedError:  "server nonexistent-server not found in profile for argument nonexistent-server.test-tool-1",
		},
		{
			name: "server not found when disabling a tool",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{"nonexistent-server.test-tool-1"},
			enableAllArgs:  []string{},
			disableAllArgs: []string{},
			expectedError:  "server nonexistent-server not found in profile for argument nonexistent-server.test-tool-1",
		},
		{
			name: "server not found in enable-all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{"nonexistent-server"},
			disableAllArgs: []string{},
			expectedError:  "server nonexistent-server not found in profile",
		},
		{
			name: "server not found in disable-all",
			servers: []db.Server{
				makeServer("test-server", []catalog.Tool{{Name: "test-tool-1"}}, []string{"test-tool-1"}),
			},
			enableArgs:     []string{},
			disableArgs:    []string{},
			enableAllArgs:  []string{},
			disableAllArgs: []string{"nonexistent-server"},
			expectedError:  "server nonexistent-server not found in profile",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dao := setupTestDB(t)
			ctx := t.Context()

			err := dao.CreateWorkingSet(ctx, db.WorkingSet{
				ID:      "test-set",
				Name:    "Test Working Set",
				Servers: tt.servers,
				Secrets: db.SecretMap{},
			})
			require.NoError(t, err)

			err = UpdateTools(ctx, dao, "test-set", tt.enableArgs, tt.disableArgs, tt.enableAllArgs, tt.disableAllArgs)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

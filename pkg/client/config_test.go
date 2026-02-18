package client

import (
	"embed"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/*
var testData embed.FS

func Test_yq_list(t *testing.T) {
	config := ReadConfig()
	tests := []struct {
		name    string
		cfg     any
		content string
		result  *MCPJSONLists
	}{
		{
			name:    "Cursor",
			cfg:     config.System[vendorCursor],
			content: "list/cursor.json",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "MCP_DOCKER",
						Command: "docker",
						Args:    []string{"mcp", "gateway", "run"},
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "Claude Desktop",
			cfg:     config.System[vendorClaudeDesktop],
			content: "list/claude-desktop.json",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "MCP_DOCKER",
						Command: "docker",
						Args:    []string{"mcp", "gateway", "run"},
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "Continue.dev",
			cfg:     config.System[vendorContinueDev],
			content: "list/continue-dev.yml",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "My MCP Server",
						Command: "uvx",
						Args:    []string{"mcp-server-sqlite", "--db-path", "/Users/NAME/test.db"},
					},
					{
						Name: "my-server",
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "VSCode",
			cfg:     config.Project[vendorVSCode],
			content: "list/vscode.json",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "Perplexity",
						Command: "docker",
						Args:    []string{"run", "-i", "--rm", "-e", "PERPLEXITY_API_KEY", "mcp/perplexity-ask"},
						Env:     map[string]string{"PERPLEXITY_API_KEY": "${input:perplexity-key}"},
					},
					{
						Name:    "fetch",
						Command: "uvx",
						Args:    []string{"mcp-server-fetch"},
					},
				},
				SSEServers: []MCPServerSSE{
					{
						Name:    "my-remote-server",
						URL:     "http://api.contoso.com/sse",
						Headers: map[string]string{"VERSION": "1.2"},
					},
				},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "Zed",
			cfg:     config.System[vendorZed],
			content: "list/zed.jsonc",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "MCP_DOCKER",
						Command: "docker",
						Args:    []string{"mcp", "gateway", "run"},
					},
					{
						Name:    "sqlite-server",
						Command: "uvx",
						Args:    []string{"mcp-server-sqlite", "--db-path", "/Users/moby/test.db"},
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "Kiro",
			cfg:     config.System[vendorKiro],
			content: "list/kiro.json",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "MCP_DOCKER",
						Command: "docker",
						Args:    []string{"mcp", "gateway", "run"},
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
		{
			name:    "OpenCode",
			cfg:     config.System["opencode"],
			content: "list/opencode.json",
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{
						Name:    "MCP_DOCKER",
						Command: "docker",
						Args:    []string{"mcp", "gateway", "run", "--profile", "test"},
					},
				},
				SSEServers:  []MCPServerSSE{},
				HTTPServers: []MCPServerHTTP{},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := getYQProcessor(t, tc.cfg)
			result, err := p.Parse(readTestData(t, tc.content))
			if tc.result == nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, *tc.result, *result)
		})
	}
}

func readTestData(t *testing.T, path string) []byte {
	t.Helper()
	file := "testdata/" + path
	content, err := testData.ReadFile(file)
	if err != nil {
		t.Fatalf("could not read test file %q: %v", file, err)
	}
	return content
}

func Test_yq_add_del(t *testing.T) {
	config := ReadConfig()
	tests := []struct {
		name     string
		cfg      any
		original string
		afterAdd string
		afterDel string
	}{
		{
			name:     "Continue.dev - append",
			cfg:      config.System[vendorContinueDev],
			original: "continue-dev-append/original.yml",
			afterAdd: "continue-dev-append/after-add.yml",
			afterDel: "continue-dev-append/after-del.yml",
		},
		{
			name:     "Continue.dev - create",
			cfg:      config.System[vendorContinueDev],
			original: "continue-dev-create/original.yml",
			afterAdd: "continue-dev-create/after-add.yml",
			afterDel: "continue-dev-create/after-del.yml",
		},
		{
			name:     "Claude Desktop - append",
			cfg:      config.System[vendorClaudeDesktop],
			original: "claude-desktop-append/original.json",
			afterAdd: "claude-desktop-append/after-add.json",
			afterDel: "claude-desktop-append/after-del.json",
		},
		{
			name:     "Claude Desktop - create",
			cfg:      config.System[vendorClaudeDesktop],
			original: "claude-desktop-create/original.json",
			afterAdd: "claude-desktop-create/after-add.json",
			afterDel: "claude-desktop-create/after-del.json",
		},
		{
			name:     "VSCode - append",
			cfg:      config.Project[vendorVSCode],
			original: "vscode-append/original.json",
			afterAdd: "vscode-append/after-add.json",
			afterDel: "vscode-append/after-del.json",
		},
		{
			name:     "VSCode - create",
			cfg:      config.Project[vendorVSCode],
			original: "vscode-create/original.json",
			afterAdd: "vscode-create/after-add.json",
			afterDel: "vscode-create/after-del.json",
		},
		{
			name:     "Zed - append",
			cfg:      config.System[vendorZed],
			original: "zed-append/original.jsonc",
			afterAdd: "zed-append/after-add.json",
			afterDel: "zed-append/after-del.json",
		},
		{
			name: "Zed - create",
			cfg:  config.System[vendorZed],
			// The real configuation file is .json and nothing rewrites
			// the file extension. The .jsonc extension is only used so
			// that IDEs do not complain that comments are invalid .json
			original: "zed-create/original.jsonc",
			afterAdd: "zed-create/after-add.json",
			afterDel: "zed-create/after-del.json",
		},
		{
			name:     "Kiro - append",
			cfg:      config.System[vendorKiro],
			original: "kiro-append/original.json",
			afterAdd: "kiro-append/after-add.json",
			afterDel: "kiro-append/after-del.json",
		},
		{
			name:     "Kiro - create",
			cfg:      config.System[vendorKiro],
			original: "kiro-create/original.json",
			afterAdd: "kiro-create/after-add.json",
			afterDel: "kiro-create/after-del.json",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := getYQProcessor(t, tc.cfg)
			original := readTestData(t, filepath.Join("add_del", tc.original))
			if len(original) == 0 {
				afterDelFromEmpty, err := p.Del([]byte{}, "my-server")
				require.NoError(t, err)
				assert.Empty(t, string(afterDelFromEmpty))
			}
			result, err := p.Add(original, MCPServerSTDIO{
				Name:    "my-server",
				Command: "docker",
				Args:    []string{"mcp", "gateway", "run"},
			})
			require.NoError(t, err)
			assert.Equal(t, string(readTestData(t, filepath.Join("add_del", tc.afterAdd))), string(result))
			afterDel, err := p.Del(result, "my-server")
			require.NoError(t, err)
			assert.Equal(t, string(readTestData(t, filepath.Join("add_del", tc.afterDel))), string(afterDel))
			afterDel2, err := p.Del(result, "my-server")
			require.NoError(t, err)
			assert.Equal(t, string(readTestData(t, filepath.Join("add_del", tc.afterDel))), string(afterDel2))
		})
	}
}

func getYQProcessor(t *testing.T, cfg any) yqProcessor {
	t.Helper()
	switch e := cfg.(type) {
	case globalCfg:
		temp, err := NewGlobalCfgProcessor(e)
		require.NoError(t, err)
		return temp.p
	case localCfg:
		temp, err := NewLocalCfgProcessor(e, "")
		require.NoError(t, err)
		return temp.p
	default:
		t.Fatalf("unknown cfg type: %T", cfg)
		return yqProcessor{}
	}
}

func TestFindClientsByProfile(t *testing.T) {
	tests := []struct {
		name            string
		profileID       string
		mockConfigs     map[string][]byte
		expectedVendors []string
	}{
		{
			name:      "finds all clients with matching profile",
			profileID: "test-profile",
			mockConfigs: map[string][]byte{
				vendorCursor:        readTestData(t, "find-profiles/cursor-with-profile.json"),
				vendorClaudeDesktop: readTestData(t, "find-profiles/claude-desktop-with-profile.json"),
				vendorZed:           readTestData(t, "find-profiles/zed-with-profile.json"),
				vendorKiro:          readTestData(t, "find-profiles/kiro-with-profile.json"),
				vendorContinueDev:   readTestData(t, "find-profiles/continue-with-profile.yml"),
			},
			expectedVendors: []string{vendorCursor, vendorClaudeDesktop, vendorZed, vendorKiro, vendorContinueDev},
		},
		{
			name:      "finds no clients when profile doesn't match",
			profileID: "non-existent-profile",
			mockConfigs: map[string][]byte{
				vendorCursor:        readTestData(t, "find-profiles/cursor-with-profile.json"),
				vendorClaudeDesktop: readTestData(t, "find-profiles/claude-desktop-with-profile.json"),
				vendorZed:           readTestData(t, "find-profiles/zed-without-profile.json"),
				vendorKiro:          readTestData(t, "find-profiles/kiro-without-profile.json"),
				vendorContinueDev:   readTestData(t, "find-profiles/continue-without-profile.yml"),
			},
			expectedVendors: []string{},
		},
		{
			name:      "find clients with default profile",
			profileID: "default",
			mockConfigs: map[string][]byte{
				vendorCursor:        readTestData(t, "find-profiles/cursor-with-profile.json"),
				vendorClaudeDesktop: readTestData(t, "find-profiles/claude-desktop-with-profile.json"),
				vendorZed:           readTestData(t, "find-profiles/zed-without-profile.json"),
				vendorKiro:          readTestData(t, "find-profiles/kiro-without-profile.json"),
				vendorContinueDev:   readTestData(t, "find-profiles/continue-without-profile.yml"),
			},
			expectedVendors: []string{vendorZed, vendorKiro, vendorContinueDev},
		},
		{
			name:      "finds clients without profile when searching for empty string",
			profileID: "",
			mockConfigs: map[string][]byte{
				vendorCursor:        readTestData(t, "find-profiles/cursor-with-profile.json"),
				vendorClaudeDesktop: readTestData(t, "find-profiles/claude-desktop-without-profile.json"),
				vendorZed:           readTestData(t, "find-profiles/zed-without-profile.json"),
				vendorKiro:          readTestData(t, "find-profiles/kiro-without-profile.json"),
				vendorContinueDev:   readTestData(t, "find-profiles/continue-without-profile.yml"),
			},
			expectedVendors: []string{vendorClaudeDesktop, vendorZed, vendorKiro, vendorContinueDev},
		},
		{
			name:      "finds mix of clients with and without matching profile",
			profileID: "test-profile",
			mockConfigs: map[string][]byte{
				vendorCursor:        readTestData(t, "find-profiles/cursor-with-profile.json"),
				vendorClaudeDesktop: readTestData(t, "find-profiles/claude-desktop-without-profile.json"),
				vendorZed:           readTestData(t, "find-profiles/zed-with-profile.json"),
				vendorKiro:          readTestData(t, "find-profiles/kiro-without-profile.json"),
				vendorContinueDev:   readTestData(t, "find-profiles/continue-with-profile.yml"),
			},
			expectedVendors: []string{vendorCursor, vendorZed, vendorContinueDev},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := ReadConfig()

			expectedMap := make(map[string]bool)
			for _, vendor := range tc.expectedVendors {
				expectedMap[vendor] = true
			}

			foundClients := 0
			for vendor, configData := range tc.mockConfigs {
				pathCfg, ok := config.System[vendor]
				if !ok {
					continue
				}

				processor, err := NewGlobalCfgProcessor(pathCfg)
				require.NoError(t, err)

				lists, err := processor.p.Parse(configData)
				require.NoError(t, err)

				clientCfg := MCPClientCfg{
					MCPClientCfgBase: MCPClientCfgBase{
						DisplayName: pathCfg.DisplayName,
						Source:      pathCfg.Source,
						Icon:        pathCfg.Icon,
					},
					IsInstalled:   true,
					IsOsSupported: true,
				}
				clientCfg.setParseResult(lists, nil)

				if matchesProfile(clientCfg, tc.profileID) {
					clientCfg.WorkingSet = tc.profileID // Overwrite if matching
				}

				shouldMatch := expectedMap[vendor]

				if shouldMatch {
					assert.Equal(t, tc.profileID, clientCfg.WorkingSet,
						"Expected vendor %s to have profile %s", vendor, tc.profileID)
					foundClients++
				} else {
					assert.NotEqual(t, tc.profileID, clientCfg.WorkingSet,
						"Expected vendor %s to NOT have profile %s", vendor, tc.profileID)
				}

				if clientCfg.WorkingSet != "" || clientCfg.IsMCPCatalogConnected {
					assert.True(t, clientCfg.IsMCPCatalogConnected,
						"IsMCPCatalogConnected should be true when MCP_DOCKER is configured")
				}
			}

			assert.Equal(t, len(tc.expectedVendors), foundClients,
				"Should find exactly %d clients with profile %s", len(tc.expectedVendors), tc.profileID)
		})
	}
}

func TestIsSupportedMCPClient(t *testing.T) {
	config := ReadConfig()

	tests := []struct {
		name     string
		vendor   string
		global   bool
		expected bool
	}{
		// Valid global (system) vendors
		{
			name:     "cursor is supported as global",
			vendor:   vendorCursor,
			global:   true,
			expected: true,
		},
		{
			name:     "vscode is supported as global",
			vendor:   vendorVSCode,
			global:   true,
			expected: true, // vscode is in both System and Project
		},
		{
			name:     "claude-desktop is supported as global",
			vendor:   vendorClaudeDesktop,
			global:   true,
			expected: true,
		},
		{
			name:     "continue is supported as global",
			vendor:   vendorContinueDev,
			global:   true,
			expected: true,
		},
		{
			name:     "zed is supported as global",
			vendor:   vendorZed,
			global:   true,
			expected: true,
		},
		{
			name:     "kiro is supported as global",
			vendor:   vendorKiro,
			global:   true,
			expected: true,
		},
		{
			name:     "gordon is supported as global",
			vendor:   VendorGordon,
			global:   true,
			expected: true,
		},
		{
			name:     "codex is supported as global",
			vendor:   VendorCodex,
			global:   true,
			expected: true,
		},
		// Valid project (local) vendors
		{
			name:     "vscode is supported as project",
			vendor:   vendorVSCode,
			global:   false,
			expected: true,
		},
		{
			name:     "cursor is supported as project",
			vendor:   vendorCursor,
			global:   false,
			expected: true, // cursor is in both System and Project
		},
		{
			name:     "kiro is supported as project",
			vendor:   vendorKiro,
			global:   false,
			expected: true, // kiro is in both System and Project
		},
		{
			name:     "gordon is not supported as project",
			vendor:   VendorGordon,
			global:   false,
			expected: false,
		},
		{
			name:     "zed is not supported as project",
			vendor:   vendorZed,
			global:   false,
			expected: false, // zed is only in System, not in Project
		},
		{
			name:     "claude-desktop is not supported as project",
			vendor:   vendorClaudeDesktop,
			global:   false,
			expected: false, // claude-desktop is only in System, not in Project
		},
		{
			name:     "codex is not supported as project",
			vendor:   VendorCodex,
			global:   false,
			expected: false,
		},
		// Invalid vendors
		{
			name:     "invalid vendor for global",
			vendor:   "invalid-vendor",
			global:   true,
			expected: false,
		},
		{
			name:     "invalid vendor for project",
			vendor:   "invalid-vendor",
			global:   false,
			expected: false,
		},
		{
			name:     "empty vendor for global",
			vendor:   "",
			global:   true,
			expected: false,
		},
		{
			name:     "empty vendor for project",
			vendor:   "",
			global:   false,
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := IsSupportedMCPClient(*config, tc.vendor, tc.global)
			assert.Equal(t, tc.expected, result)
		})
	}
}

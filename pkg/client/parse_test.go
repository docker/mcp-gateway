package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_UnmarshalMCPJSONList(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		result *MCPJSONLists
	}{
		{
			name: "valid input",
			input: `[
 {
   "command": "",
   "args": [
     "foo"
   ],
   "name": "my-server",
   "type": "stdio"
 },
 {
   "name":"my-remote-server",
   "type": "sse",
   "url": "http://api.contoso.com/sse",
   "headers": { "VERSION": "1.2" }
 },
 {
   "name":"my-remote-server",
   "type": "http",
   "url": "http://api.contoso.com/http",
   "headers": { "VERSION": "1.2" }
 }
]`,
			result: &MCPJSONLists{
				STDIOServers: []MCPServerSTDIO{
					{Name: "my-server", Args: []string{"foo"}},
				},
				SSEServers: []MCPServerSSE{
					{Name: "my-remote-server", URL: "http://api.contoso.com/sse", Headers: map[string]string{"VERSION": "1.2"}},
				},
				HTTPServers: []MCPServerHTTP{
					{Name: "my-remote-server", URL: "http://api.contoso.com/http", Headers: map[string]string{"VERSION": "1.2"}},
				},
			},
		},
		{
			name:   "empty input",
			input:  ``,
			result: &MCPJSONLists{},
		},
		{
			name:  "valid JSON but invalid format",
			input: `{"command": "foo"}`,
		},
		{
			name:  "invalid JSON",
			input: `{"command": "foo"`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := UnmarshalMCPJSONList([]byte(tc.input))
			if tc.result == nil {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.result, result)
		})
	}
}

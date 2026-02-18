package gateway

import (
	"strings"
	"testing"
)

func TestCheckDuplicateKeys(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "no duplicates",
			json:    `{"name":"test","method":"call"}`,
			wantErr: false,
		},
		{
			name:    "duplicate with different case - name and Name",
			json:    `{"name":"legitimate","Name":"smuggled"}`,
			wantErr: true,
			errMsg:  "case variants",
		},
		{
			name:    "duplicate in nested object",
			json:    `{"name":"test","arguments":{"key":"value","Key":"smuggled"}}`,
			wantErr: true,
			errMsg:  "case variants",
		},
		{
			name:    "duplicate in array element",
			json:    `{"items":[{"name":"a","Name":"b"}]}`,
			wantErr: true,
			errMsg:  "case variants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDuplicateKeys([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("checkDuplicateKeys() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("checkDuplicateKeys() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("checkDuplicateKeys() unexpected error = %v", err)
			}
		})
	}
}

func TestValidateJSONStructure(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		json    string
		wantErr bool
	}{
		{
			name:    "valid tools/call",
			method:  "tools/call",
			json:    `{"name":"greet","arguments":{}}`,
			wantErr: false,
		},
		{
			name:    "attack: duplicate name in tools/call",
			method:  "tools/call",
			json:    `{"name":"greet","Name":"secretTool"}`,
			wantErr: true,
		},
		{
			name:    "attack: duplicate in nested arguments",
			method:  "tools/call",
			json:    `{"name":"test","arguments":{"key":"a","Key":"b"}}`,
			wantErr: true,
		},
		{
			name:    "valid prompts/get",
			method:  "prompts/get",
			json:    `{"name":"test"}`,
			wantErr: false,
		},
		{
			name:    "attack: duplicate name in prompts/get",
			method:  "prompts/get",
			json:    `{"name":"legitimate","Name":"smuggled"}`,
			wantErr: true,
		},
		{
			name:    "valid resources/read",
			method:  "resources/read",
			json:    `{"uri":"file:///path"}`,
			wantErr: false,
		},
		{
			name:    "attack: duplicate uri in resources/read",
			method:  "resources/read",
			json:    `{"uri":"file:///allowed","URI":"file:///secret"}`,
			wantErr: true,
		},
		{
			name:    "case variants allowed when no duplicates",
			method:  "tools/call",
			json:    `{"Name":"tool"}`,
			wantErr: false,
		},
		{
			name:    "unknown fields are allowed",
			method:  "tools/call",
			json:    `{"name":"tool","unknownField":"value"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONStructure(tt.method, []byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateJSONStructure() expected error, got nil")
				}
			} else if err != nil {
				t.Errorf("validateJSONStructure() unexpected error = %v", err)
			}
		})
	}
}

// TestValidateJSONMiddleware_Logic tests the core duplicate key detection logic
func TestValidateJSONMiddleware_Logic(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		json    string
		wantErr bool
	}{
		{
			name:    "blocks attack with duplicate keys",
			method:  "tools/call",
			json:    `{"name":"greet","Name":"secretTool"}`,
			wantErr: true,
		},
		{
			name:    "allows legitimate request",
			method:  "tools/call",
			json:    `{"name":"greet","arguments":{"user":"Alice"}}`,
			wantErr: false,
		},
		{
			name:    "blocks duplicate in nested arguments",
			method:  "tools/call",
			json:    `{"name":"test","arguments":{"key":"a","Key":"b"}}`,
			wantErr: true,
		},
		{
			name:    "allows case variant when no duplicates",
			method:  "tools/call",
			json:    `{"Name":"greet"}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONStructure(tt.method, []byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Error("validateJSONStructure() should reject attack payload, got nil error")
				}
			} else if err != nil {
				t.Errorf("validateJSONStructure() should allow legitimate request, got error: %v", err)
			}
		})
	}
}

func TestNeedsValidation(t *testing.T) {
	tests := []struct {
		method string
		want   bool
	}{
		{"tools/call", true},
		{"prompts/get", true},
		{"resources/read", true},
		{"tools/list", false},
		{"prompts/list", false},
		{"initialize", false},
		{"ping", false},
	}

	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			if got := needsValidation(tt.method); got != tt.want {
				t.Errorf("needsValidation(%q) = %v, want %v", tt.method, got, tt.want)
			}
		})
	}
}

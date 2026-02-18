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

func TestCheckFieldNames(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		json    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid tools/call",
			method:  "tools/call",
			json:    `{"name":"greet","arguments":{}}`,
			wantErr: false,
		},
		{
			name:    "wrong case in tools/call - Name",
			method:  "tools/call",
			json:    `{"Name":"greet"}`,
			wantErr: true,
			errMsg:  "wrong case",
		},
		{
			name:    "valid prompts/get",
			method:  "prompts/get",
			json:    `{"name":"test"}`,
			wantErr: false,
		},
		{
			name:    "wrong case in prompts/get - Name",
			method:  "prompts/get",
			json:    `{"Name":"test"}`,
			wantErr: true,
			errMsg:  "wrong case",
		},
		{
			name:    "valid resources/read",
			method:  "resources/read",
			json:    `{"uri":"file:///path"}`,
			wantErr: false,
		},
		{
			name:    "wrong case in resources/read - URI",
			method:  "resources/read",
			json:    `{"URI":"file:///path"}`,
			wantErr: true,
			errMsg:  "wrong case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkFieldNames(tt.method, []byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("checkFieldNames() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("checkFieldNames() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("checkFieldNames() unexpected error = %v", err)
			}
		})
	}
}

func TestCheckNestedParams(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid nested arguments",
			json:    `{"name":"test","arguments":{"param":"value"}}`,
			wantErr: false,
		},
		{
			name:    "duplicate in arguments",
			json:    `{"name":"test","arguments":{"param":"value","Param":"smuggled"}}`,
			wantErr: true,
			errMsg:  "case variants",
		},
		{
			name:    "deeply nested duplicate",
			json:    `{"name":"test","arguments":{"nested":{"key":"value","Key":"smuggled"}}}`,
			wantErr: true,
			errMsg:  "case variants",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkNestedParams([]byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("checkNestedParams() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("checkNestedParams() error = %v, want error containing %v", err, tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("checkNestedParams() unexpected error = %v", err)
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
			name:    "attack: wrong case in tools/call",
			method:  "tools/call",
			json:    `{"Name":"secretTool"}`,
			wantErr: true,
		},
		{
			name:    "attack: duplicate in arguments",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONStructure(tt.method, []byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateJSONStructure() expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("validateJSONStructure() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateJSONMiddleware tests the middleware by testing the validation
// functions it uses directly, since we can't easily mock the mcp.Request interface
// due to unexported methods.
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
			name:    "blocks attack with wrong case",
			method:  "tools/call",
			json:    `{"Name":"secretTool"}`,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJSONStructure(tt.method, []byte(tt.json))
			if tt.wantErr {
				if err == nil {
					t.Error("validateJSONStructure() should reject attack payload, got nil error")
				}
			} else {
				if err != nil {
					t.Errorf("validateJSONStructure() should allow legitimate request, got error: %v", err)
				}
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

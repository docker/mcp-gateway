package catalognext

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/remote/transport"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "manifest unknown",
			err:      &transport.Error{Errors: []transport.Diagnostic{{Code: transport.ManifestUnknownErrorCode}}},
			expected: true,
		},
		{
			name:     "name unknown",
			err:      &transport.Error{Errors: []transport.Diagnostic{{Code: transport.NameUnknownErrorCode}}},
			expected: true,
		},
		{
			name:     "wrapped manifest unknown",
			err:      fmt.Errorf("fetch failed: %w", &transport.Error{Errors: []transport.Diagnostic{{Code: transport.ManifestUnknownErrorCode}}}),
			expected: true,
		},
		{
			name:     "unauthorized error",
			err:      &transport.Error{Errors: []transport.Diagnostic{{Code: transport.UnauthorizedErrorCode}}},
			expected: false,
		},
		{
			name:     "non-transport error",
			err:      fmt.Errorf("network timeout"),
			expected: false,
		},
		{
			name:     "transport error with no diagnostics",
			err:      &transport.Error{StatusCode: http.StatusNotFound},
			expected: false,
		},
		{
			name: "multiple diagnostics with one match",
			err: &transport.Error{Errors: []transport.Diagnostic{
				{Code: transport.UnauthorizedErrorCode},
				{Code: transport.ManifestUnknownErrorCode},
			}},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isNotFoundError(tt.err))
		})
	}
}

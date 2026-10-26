package gateway

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMergeResolvedSecrets(t *testing.T) {
	tests := []struct {
		name     string
		existing map[string]string
		incoming map[string]string
		expected map[string]string
	}{
		{
			name:     "does not downgrade a concrete value to an se:// reference",
			existing: map[string]string{"TOKEN": "real-token"},
			incoming: map[string]string{"TOKEN": "se://TOKEN"},
			expected: map[string]string{"TOKEN": "real-token"},
		},
		{
			name:     "adds a reference when no value exists yet",
			existing: map[string]string{},
			incoming: map[string]string{"TOKEN": "se://TOKEN"},
			expected: map[string]string{"TOKEN": "se://TOKEN"},
		},
		{
			name:     "refreshes an existing se:// reference",
			existing: map[string]string{"TOKEN": "se://old"},
			incoming: map[string]string{"TOKEN": "se://new"},
			expected: map[string]string{"TOKEN": "se://new"},
		},
		{
			name:     "overwrites an empty value",
			existing: map[string]string{"TOKEN": ""},
			incoming: map[string]string{"TOKEN": "se://TOKEN"},
			expected: map[string]string{"TOKEN": "se://TOKEN"},
		},
		{
			name:     "keeps concrete value but adds unrelated reference",
			existing: map[string]string{"TOKEN": "real-token"},
			incoming: map[string]string{"TOKEN": "se://TOKEN", "OTHER": "se://OTHER"},
			expected: map[string]string{"TOKEN": "real-token", "OTHER": "se://OTHER"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Configuration{secrets: tt.existing}
			c.mergeResolvedSecrets(tt.incoming)
			assert.Equal(t, tt.expected, c.secrets)
		})
	}
}

func TestMergeResolvedSecretsInitializesNilMap(t *testing.T) {
	c := &Configuration{}
	c.mergeResolvedSecrets(map[string]string{"TOKEN": "se://TOKEN"})
	assert.Equal(t, map[string]string{"TOKEN": "se://TOKEN"}, c.secrets)
}

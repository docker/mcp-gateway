package oauth

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateManager_Generate(t *testing.T) {
	mgr := NewStateManager()

	state1 := mgr.Generate("server1", "verifier1")
	state2 := mgr.Generate("server2", "verifier2")

	assert.NotEqual(t, state1, state2, "states should be unique")
	assert.NotEmpty(t, state1)
	assert.NotEmpty(t, state2)
}

func TestStateManager_Validate(t *testing.T) {
	tests := []struct {
		name          string
		serverName    string
		verifier      string
		expectedError bool
	}{
		{
			name:          "valid state",
			serverName:    "notion-remote",
			verifier:      "test-verifier-123",
			expectedError: false,
		},
		{
			name:          "another valid state",
			serverName:    "github-server",
			verifier:      "another-verifier-456",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := NewStateManager()

			state := mgr.Generate(tt.serverName, tt.verifier)

			serverName, verifier, err := mgr.Validate(state)
			require.NoError(t, err)
			assert.Equal(t, tt.serverName, serverName)
			assert.Equal(t, tt.verifier, verifier)
		})
	}
}

func TestStateManager_ValidateInvalid(t *testing.T) {
	mgr := NewStateManager()

	// Test with invalid state
	_, _, err := mgr.Validate("invalid-uuid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state parameter")
}

func TestStateManager_SingleUse(t *testing.T) {
	mgr := NewStateManager()

	state := mgr.Generate("test-server", "test-verifier")

	// First validation should succeed
	serverName, verifier, err := mgr.Validate(state)
	require.NoError(t, err)
	assert.Equal(t, "test-server", serverName)
	assert.Equal(t, "test-verifier", verifier)

	// Second validation should fail (single-use)
	_, _, err = mgr.Validate(state)
	require.Error(t, err, "state should be invalid after first use")
	assert.Contains(t, err.Error(), "invalid state parameter")
}

func TestStateManager_Concurrent(t *testing.T) {
	mgr := NewStateManager()

	// Test thread safety with goroutines
	const numGoroutines = 100
	var wg sync.WaitGroup
	states := make([]string, numGoroutines)

	// Generate states concurrently
	for i := range numGoroutines {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			states[idx] = mgr.Generate("server", "verifier")
		}(i)
	}
	wg.Wait()

	// Verify all states are unique
	stateSet := make(map[string]bool)
	for _, state := range states {
		assert.NotEmpty(t, state)
		assert.False(t, stateSet[state], "duplicate state found: %s", state)
		stateSet[state] = true
	}

	// Validate states concurrently
	results := make(chan error, numGoroutines)
	for i := range numGoroutines {
		wg.Add(1)
		go func(state string) {
			defer wg.Done()
			_, _, err := mgr.Validate(state)
			results <- err
		}(states[i])
	}
	wg.Wait()
	close(results)

	// All validations should succeed
	for err := range results {
		assert.NoError(t, err)
	}
}

func TestStateManager_Clear(t *testing.T) {
	mgr := NewStateManager()

	state := mgr.Generate("test-server", "test-verifier")

	// Clear the state
	mgr.Clear(state)

	// Validation should fail
	_, _, err := mgr.Validate(state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid state parameter")
}

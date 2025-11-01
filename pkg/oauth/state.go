package oauth

import (
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// StateManager manages OAuth state parameters and PKCE verifiers
// States and verifiers are stored in memory and cleared after use
type StateManager struct {
	mu        sync.RWMutex
	states    map[string]string // state -> serverName
	verifiers map[string]string // state -> PKCE verifier
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		states:    make(map[string]string),
		verifiers: make(map[string]string),
	}
}

// Generate creates a new state parameter and stores the associated server name and PKCE verifier
// Returns the state UUID
func (s *StateManager) Generate(serverName string, verifier string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := uuid.New().String()
	s.states[state] = serverName
	s.verifiers[state] = verifier

	return state
}

// Validate checks if a state parameter is valid and returns the associated server name and verifier
// The state and verifier are removed after validation (single-use)
func (s *StateManager) Validate(state string) (serverName string, verifier string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	serverName, ok := s.states[state]
	if !ok {
		return "", "", fmt.Errorf("invalid state parameter")
	}

	verifier, ok = s.verifiers[state]
	if !ok {
		return "", "", fmt.Errorf("PKCE verifier not found for state")
	}

	// Remove after validation (single-use)
	delete(s.states, state)
	delete(s.verifiers, state)

	return serverName, verifier, nil
}

// Clear removes a state and its associated verifier without validation
// Useful for cleanup on errors
func (s *StateManager) Clear(state string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.states, state)
	delete(s.verifiers, state)
}

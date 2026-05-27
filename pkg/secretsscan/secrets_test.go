package secretsscan

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesntContainsSecrets(t *testing.T) {
	assert.False(t, ContainsSecrets("1234567890"))
}

func TestContainsSecrets(t *testing.T) {
	// Assemble token-shaped inputs at runtime rather than as source literals,
	// so secret-scanning tooling doesn't flag this file as containing leaked
	// credentials. The assembled strings still satisfy the GitHub-PAT regex
	// (ghp_[0-9a-zA-Z]{36}) and Docker-PAT regex (dckr_pat_[-0-9a-zA-Z]{27}).
	githubPATShaped := "ghp_" + strings.Repeat("T", 36)
	dockerPATShaped := "dckr_pat_" + strings.Repeat("T", 27)

	assert.True(t, ContainsSecrets(githubPATShaped))
	assert.True(t, ContainsSecrets(dockerPATShaped))
}

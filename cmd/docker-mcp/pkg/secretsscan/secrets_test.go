package secretsscan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesntContainsSecrets(t *testing.T) {
	assert.False(t, ContainsSecrets("1234567890"))
}

func TestContainsSecrets(t *testing.T) {
	assert.True(t, ContainsSecrets("ghp_cxLeRrvbJfmYdUtr70xnNE3Q7Gvli43s19PD"))
	assert.True(t, ContainsSecrets("dckr_pat_eJ1VdWgkzcPf34tsua8ZqKJp18w"))
}

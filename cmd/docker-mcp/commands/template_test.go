package commands

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateCommandStructure(t *testing.T) {
	cmd := templateCommand()

	assert.Equal(t, "template", cmd.Use)

	// Verify subcommands exist
	subcommands := make(map[string]bool)
	for _, sub := range cmd.Commands() {
		subcommands[sub.Name()] = true
	}
	assert.True(t, subcommands["list"], "template list subcommand should exist")
	assert.True(t, subcommands["use"], "template use subcommand should exist")
}

func TestListTemplatesCommand(t *testing.T) {
	cmd := listTemplatesCommand()
	assert.Equal(t, "list", cmd.Use)
	assert.Contains(t, cmd.Aliases, "ls")
}

func TestUseTemplateCommandRequiresArgs(t *testing.T) {
	cmd := useTemplateCommand()
	require.NotNil(t, cmd)

	// Verify the command requires exactly 1 argument
	err := cmd.Args(cmd, []string{})
	require.Error(t, err, "use command should require exactly 1 argument")

	err = cmd.Args(cmd, []string{"ai-coding"})
	assert.NoError(t, err, "use command should accept 1 argument")
}

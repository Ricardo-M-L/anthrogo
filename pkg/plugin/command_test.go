package plugin

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/command"
)

func makeCmd(t string, body string) DynamicCommand {
	return DynamicCommand{
		spec: CommandSpec{
			Name:        "/test-cmd",
			Description: "a test command",
			Type:        t,
			Body:        body,
		},
	}
}

func TestDynamicCommand_LocalPrompt(t *testing.T) {
	cmd := makeCmd("local-prompt", "do the thing")
	res, err := cmd.Run(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Equal(t, "do the thing", res.SubmitText)
	assert.Empty(t, res.Text)
}

func TestDynamicCommand_Submit(t *testing.T) {
	cmd := makeCmd("submit", "submit body")
	res, err := cmd.Run(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Equal(t, "submit body", res.SubmitText)
}

func TestDynamicCommand_Local(t *testing.T) {
	cmd := makeCmd("local", "local body")
	res, err := cmd.Run(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Equal(t, "local body", res.Text)
	assert.Empty(t, res.SubmitText)
}

func TestDynamicCommand_UnknownType(t *testing.T) {
	cmd := makeCmd("weird", "whatever")
	res, err := cmd.Run(context.Background(), "", nil)
	require.NoError(t, err)
	assert.Contains(t, res.Text, "unknown type")
	assert.Contains(t, res.Text, "weird")
}

func TestDynamicCommand_ArgsAppended(t *testing.T) {
	cmd := makeCmd("local-prompt", "base body")
	res, err := cmd.Run(context.Background(), "extra args", nil)
	require.NoError(t, err)
	assert.Contains(t, res.SubmitText, "base body")
	assert.Contains(t, res.SubmitText, "extra args")
}

func TestDynamicCommand_EmptyArgsNotAppended(t *testing.T) {
	cmd := makeCmd("local-prompt", "base body")
	res, err := cmd.Run(context.Background(), "   ", nil)
	require.NoError(t, err)
	assert.Equal(t, "base body", res.SubmitText)
}

func TestDynamicCommand_Metadata(t *testing.T) {
	cmd := DynamicCommand{
		spec: CommandSpec{
			Name:        "/foo",
			Aliases:     []string{"/f"},
			Description: "my desc",
			Type:        "local",
			Body:        "body",
		},
	}
	assert.Equal(t, "/foo", cmd.Name())
	assert.Equal(t, []string{"/f"}, cmd.Aliases())
	assert.Contains(t, cmd.Description(), "my desc")
	assert.Contains(t, cmd.Description(), "(plugin)")
	assert.Equal(t, command.TypeLocal, cmd.Type())
}

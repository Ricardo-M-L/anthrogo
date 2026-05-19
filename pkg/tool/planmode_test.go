package tool

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ricardo/anthrogo/pkg/permissions"
)

func TestEnterPlanMode_StashesPriorMode(t *testing.T) {
	perms := permissions.Empty()
	perms.Mode = permissions.ModeDefault
	tcx := &Context{Permissions: perms}
	_, err := (EnterPlanMode{}).Call(context.Background(), map[string]any{}, tcx)
	require.NoError(t, err)
	require.Equal(t, permissions.ModePlan, perms.Mode)
	require.Equal(t, permissions.ModeDefault, perms.PrePlanMode)
}

func TestExitPlanMode_RestoresPriorMode(t *testing.T) {
	perms := permissions.Empty()
	perms.Mode = permissions.ModePlan
	perms.PrePlanMode = permissions.ModeAcceptEdits
	tcx := &Context{Permissions: perms}
	_, err := (ExitPlanMode{}).Call(context.Background(), map[string]any{}, tcx)
	require.NoError(t, err)
	require.Equal(t, permissions.ModeAcceptEdits, perms.Mode)
}

func TestEnterPlanMode_NilPermissions_Errors(t *testing.T) {
	res, _ := (EnterPlanMode{}).Call(context.Background(), map[string]any{}, &Context{})
	require.True(t, res.IsError)
}

package tui

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderPlanBadge_Hidden(t *testing.T) {
	require.Equal(t, "", renderPlanBadge(DefaultTheme(), false))
}

func TestRenderPlanBadge_Visible(t *testing.T) {
	require.Contains(t, renderPlanBadge(DefaultTheme(), true), "PLAN MODE")
}

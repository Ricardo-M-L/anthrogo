package session

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecord_SchemaVersion_PreservedAcrossWriteRead(t *testing.T) {
	rec := Record{Kind: KindSessionMeta, SessionMeta: &SessionMeta{
		SessionID: "x", SchemaVersion: 2, CreatedAt: time.Now(),
	}}
	line, err := rec.MarshalJSONLine()
	require.NoError(t, err)
	parsed, err := UnmarshalJSONLine(line)
	require.NoError(t, err)
	require.Equal(t, 2, parsed.SessionMeta.SchemaVersion)
}

package tool

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/ricardo/anthrogo/pkg/permissions"
	"github.com/stretchr/testify/require"
)

func TestSQLQuery_SQLiteSelect(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE users (id INTEGER, name TEXT)")
	require.NoError(t, err)
	_, err = db.Exec("INSERT INTO users VALUES (1, 'alice'), (2, 'bob')")
	require.NoError(t, err)
	db.Close()

	res, err := (&SQLQuery{}).Call(context.Background(), map[string]any{
		"driver": "sqlite",
		"dsn":    dbPath,
		"query":  "SELECT id, name FROM users ORDER BY id",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "alice")
	require.Contains(t, res.Text, "bob")
	require.Equal(t, 2, res.Data["row_count"])
}

func TestSQLQuery_SQLiteInsert(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	require.NoError(t, err)
	db.Close()

	res, err := (&SQLQuery{}).Call(context.Background(), map[string]any{
		"driver": "sqlite",
		"dsn":    dbPath,
		"query":  "INSERT INTO users (name) VALUES (?)",
		"params": []any{"charlie"},
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Contains(t, res.Text, "rows_affected=1")
}

func TestSQLQuery_PermissionReadOnly(t *testing.T) {
	d := (&SQLQuery{}).Permission(context.Background(), map[string]any{"query": "SELECT * FROM x"})
	require.Equal(t, permissions.BehaviorAllow, d.Behavior)
}

func TestSQLQuery_PermissionMutating(t *testing.T) {
	d := (&SQLQuery{}).Permission(context.Background(), map[string]any{"query": "DELETE FROM x"})
	require.Equal(t, permissions.BehaviorAsk, d.Behavior)
}

func TestSQLQuery_MissingDSN(t *testing.T) {
	res, err := (&SQLQuery{}).Call(context.Background(), map[string]any{"query": "SELECT 1"}, nil)
	require.NoError(t, err)
	require.True(t, res.IsError)
}

func TestSQLQuery_EnvDSN(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (n INTEGER)")
	require.NoError(t, err)
	db.Close()
	t.Setenv("TEST_DSN", dbPath)
	res, err := (&SQLQuery{}).Call(context.Background(), map[string]any{
		"driver": "sqlite",
		"dsn":    "env:TEST_DSN",
		"query":  "SELECT * FROM t",
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
}

func TestSQLQuery_MaxRowsCap(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec("CREATE TABLE t (n INTEGER)")
	require.NoError(t, err)
	for i := 0; i < 10; i++ {
		_, err = db.Exec("INSERT INTO t VALUES (?)", i)
		require.NoError(t, err)
	}
	db.Close()

	res, err := (&SQLQuery{}).Call(context.Background(), map[string]any{
		"driver":   "sqlite",
		"dsn":      dbPath,
		"query":    "SELECT * FROM t",
		"max_rows": 3,
	}, nil)
	require.NoError(t, err)
	require.False(t, res.IsError)
	require.Equal(t, 3, res.Data["row_count"])
	require.Equal(t, true, res.Data["truncated"])
}

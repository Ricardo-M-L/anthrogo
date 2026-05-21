package tool

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/ricardo/anthrogo/pkg/permissions"
)

// SQLQuery runs a SQL query against postgres, mysql, or sqlite.
// SELECT/EXPLAIN/SHOW/DESCRIBE auto-allow; mutating queries go through the gate.
type SQLQuery struct{ DefaultPermission }

func (*SQLQuery) Permission(_ context.Context, input map[string]any) permissions.Decision {
	q, _ := input["query"].(string)
	if isReadOnlyQuery(q) {
		return permissions.Decision{Behavior: permissions.BehaviorAllow}
	}
	return permissions.Decision{Behavior: permissions.BehaviorAsk}
}

func isReadOnlyQuery(q string) bool {
	trimmed := strings.TrimSpace(q)
	if trimmed == "" {
		return false
	}
	head := strings.ToUpper(strings.Fields(trimmed)[0])
	switch head {
	case "SELECT", "EXPLAIN", "SHOW", "DESCRIBE", "DESC", "WITH":
		return true
	}
	return false
}

func (*SQLQuery) Name() string { return "SQLQuery" }

func (*SQLQuery) Description(context.Context) string {
	return "Run a SQL query against postgres / mysql / sqlite. SELECT/EXPLAIN/SHOW/DESCRIBE auto-allow; INSERT/UPDATE/DELETE/DDL go through the permission gate."
}

func (*SQLQuery) UserFacingName(input map[string]any) string {
	if q, _ := input["query"].(string); q != "" {
		head := strings.SplitN(strings.TrimSpace(q), "\n", 2)[0]
		if len(head) > 60 {
			head = head[:60] + "…"
		}
		return "SQLQuery: " + head
	}
	return "SQLQuery"
}

func (*SQLQuery) IsReadOnly() bool        { return false }
func (*SQLQuery) IsConcurrencySafe() bool { return true }

func (*SQLQuery) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"driver": map[string]any{
				"type":        "string",
				"description": "postgres | mysql | sqlite (default sqlite).",
			},
			"dsn": map[string]any{
				"type":        "string",
				"description": "DSN/connection string. Supports env:VARNAME prefix.",
			},
			"query": map[string]any{
				"type": "string",
			},
			"params": map[string]any{
				"type":        "array",
				"items":       map[string]any{},
				"description": "Positional ? / $1 / $N parameter values.",
			},
			"timeout_ms": map[string]any{
				"type":        "integer",
				"description": "Default 30000.",
			},
			"max_rows": map[string]any{
				"type":        "integer",
				"description": "Default 100. Hard cap on returned rows.",
			},
		},
		"required": []string{"dsn", "query"},
	}
}

func (*SQLQuery) Call(ctx context.Context, input map[string]any, _ *Context) (Result, error) {
	driver, _ := input["driver"].(string)
	if driver == "" {
		driver = "sqlite"
	}

	dsn, _ := input["dsn"].(string)
	if dsn == "" {
		return errResult("sqlquery: dsn required"), nil
	}
	if strings.HasPrefix(dsn, "env:") {
		dsn = os.Getenv(strings.TrimPrefix(dsn, "env:"))
		if dsn == "" {
			return errResult("sqlquery: env var resolved to empty"), nil
		}
	}

	query, _ := input["query"].(string)
	if query == "" {
		return errResult("sqlquery: query required"), nil
	}

	timeoutMs := intField(input, "timeout_ms", 30000)
	maxRows := intField(input, "max_rows", 100)

	var params []any
	if p, ok := input["params"].([]any); ok {
		params = p
	}

	queryCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return errResult("sqlquery: open: " + err.Error()), nil
	}
	defer db.Close()

	if err := db.PingContext(queryCtx); err != nil {
		return errResult("sqlquery: ping: " + err.Error()), nil
	}

	if isReadOnlyQuery(query) {
		rows, err := db.QueryContext(queryCtx, query, params...)
		if err != nil {
			return errResult("sqlquery: " + err.Error()), nil
		}
		defer rows.Close()
		return formatRows(rows, maxRows), nil
	}

	result, err := db.ExecContext(queryCtx, query, params...)
	if err != nil {
		return errResult("sqlquery: " + err.Error()), nil
	}
	affected, _ := result.RowsAffected()
	lastID, _ := result.LastInsertId()
	msg := fmt.Sprintf("rows_affected=%d last_insert_id=%d", affected, lastID)
	return Result{
		Type:   ResultText,
		Text:   msg,
		ForLLM: msg,
		Data: map[string]any{
			"rows_affected":  affected,
			"last_insert_id": lastID,
		},
	}, nil
}

func formatRows(rows *sql.Rows, maxRows int) Result {
	cols, _ := rows.Columns()
	var allRows []map[string]any
	truncated := false
	for rows.Next() {
		if len(allRows) >= maxRows {
			truncated = true
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return errResult("sqlquery: scan: " + err.Error())
		}
		row := map[string]any{}
		for i, c := range cols {
			v := vals[i]
			if bv, ok := v.([]byte); ok {
				v = string(bv)
			}
			row[c] = v
		}
		allRows = append(allRows, row)
	}
	raw, _ := json.MarshalIndent(allRows, "", "  ")
	text := string(raw)
	if truncated {
		text += fmt.Sprintf("\n[truncated at max_rows=%d]", maxRows)
	}
	return Result{
		Type:   ResultText,
		Text:   text,
		ForLLM: text,
		Data: map[string]any{
			"row_count":  len(allRows),
			"columns":    cols,
			"truncated":  truncated,
		},
	}
}

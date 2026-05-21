package serve

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/ricardo/anthrogo/internal/config"
	"github.com/ricardo/anthrogo/internal/session"
	"github.com/ricardo/anthrogo/internal/version"
	"github.com/ricardo/anthrogo/pkg/query"
)

// sessionIDRe matches safe session IDs: alphanumeric, dash, underscore, 1-128 chars.
var sessionIDRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,128}$`)

// validSessionID returns true if id is safe to use as a filesystem path component.
func validSessionID(id string) bool {
	return sessionIDRe.MatchString(id)
}

// ---- /v1/health ----

type healthResponse struct {
	OK            bool    `json:"ok"`
	Version       string  `json:"version"`
	UptimeSeconds float64 `json:"uptime_seconds"`
	InFlightChats int64   `json:"in_flight_chats"`
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		OK:            true,
		Version:       version.Version,
		UptimeSeconds: time.Since(s.startedAt).Seconds(),
		InFlightChats: s.inFlight.Load(),
	})
}

// ---- /v1/tools ----

type toolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Schema      any    `json:"schema"`
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if s.cfg.Tools == nil {
		writeJSON(w, http.StatusOK, []toolInfo{})
		return
	}
	all := s.cfg.Tools.All()
	out := make([]toolInfo, 0, len(all))
	for _, t := range all {
		desc := t.Description(r.Context())
		out = append(out, toolInfo{
			Name:        t.Name(),
			Description: desc,
			Schema:      t.Schema(),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// ---- /v1/sessions ----

type sessionSummary struct {
	ID    string    `json:"id"`
	Mtime time.Time `json:"mtime"`
}

func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	home, err := config.Home()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cannot resolve home: "+err.Error())
		return
	}
	projectsDir := filepath.Join(home, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSON(w, http.StatusOK, []sessionSummary{})
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type item struct {
		id    string
		mtime time.Time
	}
	var items []item
	for _, proj := range entries {
		if !proj.IsDir() {
			continue
		}
		dir := filepath.Join(projectsDir, proj.Name())
		files, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".jsonl") {
				continue
			}
			info, err := f.Info()
			if err != nil {
				continue
			}
			items = append(items, item{
				id:    strings.TrimSuffix(f.Name(), ".jsonl"),
				mtime: info.ModTime(),
			})
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].mtime.After(items[j].mtime)
	})
	if len(items) > 100 {
		items = items[:100]
	}

	out := make([]sessionSummary, len(items))
	for i, it := range items {
		out[i] = sessionSummary{ID: it.id, Mtime: it.mtime}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}
	path, err := findSessionFile(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}
	records, err := session.Replay(path)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "replay failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request) {
	id := pathSegment(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "missing session id")
		return
	}
	path, err := findSessionFile(id)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", id))
		return
	}
	if err := os.Remove(path); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.cache.delete(id)
	w.WriteHeader(http.StatusNoContent)
}

// ---- /v1/chat ----

type chatRequest struct {
	SessionID string          `json:"session_id"`
	Messages  json.RawMessage `json:"messages"`
	Stream    bool            `json:"stream"`
	Prompt    string          `json:"prompt"`
}

type chatSyncResponse struct {
	SessionID string `json:"session_id"`
	Text      string `json:"text"`
}

// sseEvent is the envelope for every SSE frame on /v1/chat?stream=true.
type sseEvent struct {
	Type      string `json:"type"`
	Text      string `json:"text,omitempty"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolUseID string `json:"tool_use_id,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.SessionID == "" {
		writeError(w, http.StatusBadRequest, "session_id is required")
		return
	}
	if !validSessionID(req.SessionID) {
		writeError(w, http.StatusNotFound, fmt.Sprintf("session %s not found", req.SessionID))
		return
	}
	if req.Prompt == "" {
		writeError(w, http.StatusBadRequest, "prompt is required")
		return
	}

	engine, err := s.getOrCreateEngine(req.SessionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "build engine: "+err.Error())
		return
	}

	s.inFlight.Add(1)
	defer s.inFlight.Add(-1)

	ctx := r.Context()

	if req.Stream {
		s.handleChatStream(ctx, w, req, engine)
	} else {
		s.handleChatSync(ctx, w, req, engine)
	}
}

func (s *Server) handleChatSync(ctx context.Context, w http.ResponseWriter, req chatRequest, engine *query.Engine) {
	ch := engine.SubmitMessage(ctx, req.Prompt)
	var sb strings.Builder
	for ev := range ch {
		switch ev.Kind {
		case query.KindAssistantDelta:
			sb.WriteString(ev.Text)
		case query.KindError:
			if ev.Err != nil {
				writeError(w, http.StatusInternalServerError, ev.Err.Error())
				return
			}
		}
	}
	writeJSON(w, http.StatusOK, chatSyncResponse{
		SessionID: req.SessionID,
		Text:      sb.String(),
	})
}

func (s *Server) handleChatStream(ctx context.Context, w http.ResponseWriter, req chatRequest, engine *query.Engine) {
	sseHeaders(w)
	sw := newSSEWriter(w)

	ch := engine.SubmitMessage(ctx, req.Prompt)
	for ev := range ch {
		switch ev.Kind {
		case query.KindAssistantDelta:
			_ = sw.writeJSON(sseEvent{Type: "delta", Text: ev.Text})
		case query.KindToolUseRequest:
			_ = sw.writeJSON(sseEvent{Type: "tool_use", ToolName: ev.ToolName, ToolUseID: ev.ToolUseID})
		case query.KindToolResult:
			_ = sw.writeJSON(sseEvent{Type: "tool_result", ToolUseID: ev.ToolUseID})
		case query.KindError:
			errMsg := ""
			if ev.Err != nil {
				errMsg = ev.Err.Error()
			}
			_ = sw.writeJSON(sseEvent{Type: "error", Error: errMsg})
			return
		case query.KindTurnComplete:
			_ = sw.writeJSON(sseEvent{Type: "done"})
			return
		}
	}
	// Channel closed without KindTurnComplete — still emit done.
	_ = sw.writeJSON(sseEvent{Type: "done"})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// findSessionFile searches all project subdirectories under ~/.anthrogo/projects
// for a JSONL file whose base name (without .jsonl) equals id.
func findSessionFile(id string) (string, error) {
	if !validSessionID(id) {
		return "", os.ErrNotExist
	}
	home, err := config.Home()
	if err != nil {
		return "", err
	}
	projectsDir := filepath.Join(home, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return "", err
	}
	for _, proj := range entries {
		if !proj.IsDir() {
			continue
		}
		candidate := filepath.Join(projectsDir, proj.Name(), id+".jsonl")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("session %s not found", id)
}

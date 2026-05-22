package session

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ricardo/anthrogo/internal/version"
	"github.com/ricardo/anthrogo/pkg/message"
)

type NewOptions struct {
	Cwd            string
	Model          string
	PermissionMode string
	SessionID      string
}

type Store struct {
	mu        sync.Mutex
	id        string
	path      string
	f         *os.File
	w         *bufio.Writer
	startedAt time.Time
}

func New(opts NewOptions) (*Store, error) {
	if opts.SessionID == "" {
		opts.SessionID = uuid.NewString()
	}
	path, err := SessionFile(opts.Cwd, opts.SessionID)
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := tryLockExclusive(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	s := &Store{
		id:        opts.SessionID,
		path:      path,
		f:         f,
		w:         bufio.NewWriter(f),
		startedAt: time.Now(),
	}
	meta := Record{
		Kind:      KindSessionMeta,
		Timestamp: s.startedAt,
		SessionMeta: &SessionMeta{
			SessionID:       opts.SessionID,
			Cwd:             opts.Cwd,
			Model:           opts.Model,
			PermissionMode:  opts.PermissionMode,
			AnthrogoVersion: version.Version,
			CreatedAt:       s.startedAt,
			SchemaVersion:   CurrentSchemaVersion,
		},
	}
	if err := s.Append(meta); err != nil {
		_ = f.Close()
		return nil, err
	}
	return s, nil
}

// NewSubagent creates a Store for a subagent invocation. The JSONL path is
// <parent.path-without-.jsonl>/subagents/<subagentID>.jsonl. The parent's
// directory and the subagents subdirectory are created if needed.
func NewSubagent(parent *Store, subagentID string) (*Store, error) {
	if parent == nil {
		return nil, fmt.Errorf("session: parent store is nil")
	}
	if subagentID == "" {
		return nil, fmt.Errorf("session: subagent ID required")
	}
	parentBase := strings.TrimSuffix(parent.Path(), ".jsonl")
	dir := filepath.Join(parentBase, "subagents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, subagentID+".jsonl")
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}
	return &Store{
		id:   subagentID,
		path: path,
		f:    f,
		w:    bufio.NewWriter(f),
	}, nil
}

func Resume(cwd, sessionID string) (*Store, error) {
	path, err := SessionFile(cwd, sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("session %s not found: %w", sessionID, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	// Apply the same advisory exclusive lock that New() uses, so resuming a
	// session that another anthrogo process is actively writing fails
	// cleanly instead of interleaving JSONL records on disk.
	if err := tryLockExclusive(f); err != nil {
		_ = f.Close()
		return nil, err
	}
	return &Store{
		id:   sessionID,
		path: path,
		f:    f,
		w:    bufio.NewWriter(f),
	}, nil
}

func (s *Store) ID() string   { return s.id }
func (s *Store) Path() string { return s.path }

func (s *Store) Append(r Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Timestamp.IsZero() {
		r.Timestamp = time.Now()
	}
	line, err := r.MarshalJSONLine()
	if err != nil {
		return err
	}
	if _, err := s.w.Write(line); err != nil {
		return err
	}
	// Always Flush() — pushes userland bufio bytes into the OS file so an
	// anthrogo crash mid-turn doesn't lose records sitting in bufio's
	// internal buffer. The OS page cache survives a process crash; a
	// machine crash before sync still loses these bytes, which is the
	// trade-off accepted to avoid per-record fsync overhead.
	if err := s.w.Flush(); err != nil {
		return err
	}
	// Sync() to disk only on durable boundaries — TurnComplete, Error,
	// Compact — to cap fsync frequency.
	if r.Kind == KindTurnComplete || r.Kind == KindError || r.Kind == KindCompact {
		return s.f.Sync()
	}
	return nil
}

func (s *Store) NewRecordHook() func(Record) {
	return func(r Record) {
		if err := s.Append(r); err != nil {
			fmt.Fprintf(os.Stderr, "[session] record append failed: %v\n", err)
		}
	}
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.w.Flush(); err != nil {
		_ = s.f.Close()
		return err
	}
	return s.f.Close()
}

// Replay reads a session file and returns its records in file order.
// If the first record is a KindSessionMeta with a schema_version greater than
// CurrentSchemaVersion, a warning is printed to stderr but replay continues
// best-effort. Missing or zero schema_version is treated as v1 (legacy).
func Replay(path string) ([]Record, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []Record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		r, err := UnmarshalJSONLine(sc.Bytes())
		if err != nil {
			return out, fmt.Errorf("line %d: %w", len(out)+1, err)
		}
		out = append(out, r)
	}
	if err := sc.Err(); err != nil && err != io.EOF {
		return out, err
	}
	if len(out) > 0 && out[0].Kind == KindSessionMeta && out[0].SessionMeta != nil {
		v := out[0].SessionMeta.SchemaVersion
		if v == 0 {
			v = 1
		}
		if v > CurrentSchemaVersion {
			fmt.Fprintf(os.Stderr, "[session] %s: schema_version=%d unknown (anthrogo supports %d); replaying anyway\n", path, v, CurrentSchemaVersion)
		}
	}
	return out, nil
}

// Messages projects records into a flat []message.Message suitable for
// re-seeding the engine on resume.
//
// When a KindCompact record is encountered, all previously accumulated messages
// are discarded. Resume therefore starts from the most-recent compact point
// forward — the compacted summary message will be the first record after the
// compact marker (emitted as a KindUserMessage by the compact package).
func Messages(records []Record) []message.Message {
	var (
		out              []message.Message
		pendingAssistant *message.Message
	)
	flush := func() {
		if pendingAssistant != nil {
			out = append(out, *pendingAssistant)
			pendingAssistant = nil
		}
	}
	for _, r := range records {
		switch r.Kind {
		case KindCompact:
			// Discard all accumulated messages; the next records represent the
			// post-compact state (summary + tail messages).
			flush()
			out = out[:0]
		case KindUserMessage:
			flush()
			out = append(out, message.Message{Role: message.RoleUser, Content: r.UserMessage.Content})
		case KindAssistantMessage:
			flush()
			pendingAssistant = &message.Message{Role: message.RoleAssistant, Content: r.AssistantMessage.Content}
		case KindToolResult:
			flush()
			out = append(out, message.Message{
				Role: message.RoleUser,
				Content: []message.Block{{
					Type:      message.BlockToolResult,
					ToolUseID: r.ToolResult.ToolUseID,
					Text:      r.ToolResult.Text,
					IsError:   r.ToolResult.IsError,
				}},
			})
		}
	}
	flush()
	return out
}

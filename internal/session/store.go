package session

import (
	"bufio"
	"fmt"
	"io"
	"os"
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
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
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
		},
	}
	if err := s.Append(meta); err != nil {
		_ = f.Close()
		return nil, err
	}
	return s, nil
}

func Resume(cwd, sessionID string) (*Store, error) {
	path, err := SessionFile(cwd, sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("session %s not found: %w", sessionID, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
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
	if r.Kind == KindTurnComplete || r.Kind == KindError {
		if err := s.w.Flush(); err != nil {
			return err
		}
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
	return out, nil
}

// Messages projects records into a flat []message.Message suitable for
// re-seeding the engine on resume.
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

# `github.com/ricardo/anthrogo/internal/session`

```go
package session // import "github.com/ricardo/anthrogo/internal/session"


FUNCTIONS

func Messages(records []Record) []message.Message
    Messages projects records into a flat []message.Message suitable for
    re-seeding the engine on resume.

    When a KindCompact record is encountered, all previously accumulated
    messages are discarded. Resume therefore starts from the most-recent compact
    point forward — the compacted summary message will be the first record after
    the compact marker (emitted as a KindUserMessage by the compact package).

func ProjectDir(cwd string) (string, error)
func ResolveContinue(cwd string) (string, error)
func ResolveResume(cwd, idPrefix string) (string, error)
func SessionFile(cwd, sessionID string) (string, error)

TYPES

type AssistantMessage struct {
	Content    []message.Block `json:"content"`
	StopReason string          `json:"stop_reason"`
}

type CompactRecord struct {
	OriginalCount  int `json:"original_count"`
	NewCount       int `json:"new_count"`
	OriginalTokens int `json:"original_tokens"`
	NewTokens      int `json:"new_tokens"`
	// Deprecated legacy fields — kept for reading old JSONL files.
	OriginalBytes int    `json:"original_bytes,omitempty"`
	NewBytes      int    `json:"new_bytes,omitempty"`
	Trigger       string `json:"trigger"`
}
    CompactRecord is the JSONL record emitted when Engine.Compact runs.
    OriginalTokens / NewTokens replaced OriginalBytes / NewBytes in M8.10.
    Legacy JSONLs written with original_bytes / new_bytes still unmarshal
    correctly because the old fields are retained with omitempty and the unused
    direction is harmless.

type ErrorRecord struct {
	Error  string `json:"error"`
	During string `json:"during"`
}

type Kind string

const (
	KindSessionMeta      Kind = "session_meta"
	KindUserMessage      Kind = "user_message"
	KindAssistantMessage Kind = "assistant_message"
	KindToolUseRequest   Kind = "tool_use_request"
	KindToolResult       Kind = "tool_result"
	KindTurnComplete     Kind = "turn_complete"
	KindError            Kind = "error"
	KindUsage            Kind = "usage"
	KindCompact          Kind = "compact"
	KindSubagentStart    Kind = "subagent_start"
)
type NewOptions struct {
	Cwd            string
	Model          string
	PermissionMode string
	SessionID      string
}

type PersistentCache struct {
	// Has unexported fields.
}
    PersistentCache is a SQLite-backed cache of parsed []Record keyed by (path,
    modtime). On miss or stale modtime, it falls through to Replay + stores.
    Falls back to a no-op (in-memory only) if the DB can't open.

func NewPersistentCache(dbPath string, memCap int) *PersistentCache
    NewPersistentCache opens (or creates) a SQLite DB at dbPath. memCap is the
    in-memory L1 LRU capacity. If sql.Open fails, the cache degrades to L1-only
    (logs warning to stderr).

func (c *PersistentCache) Clear()
    Clear empties both L1 and L2.

func (c *PersistentCache) Close() error
    Close closes the underlying SQLite database.

func (c *PersistentCache) Get(path string) ([]Record, error)
    Get returns parsed records: L1 → L2 (if modtime matches) → Replay.

func (c *PersistentCache) Invalidate(path string)
    Invalidate removes a single path from both L1 and L2.

func (c *PersistentCache) SizeOnDisk() int
    SizeOnDisk returns the row count in the SQLite table. 0 if no DB.

type Record struct {
	Kind      Kind      `json:"type"`
	Timestamp time.Time `json:"ts"`

	SessionMeta      *SessionMeta      `json:"session_meta,omitempty"`
	UserMessage      *UserMessage      `json:"user_message,omitempty"`
	AssistantMessage *AssistantMessage `json:"assistant_message,omitempty"`
	ToolUseRequest   *ToolUseRequest   `json:"tool_use_request,omitempty"`
	ToolResult       *ToolResult       `json:"tool_result,omitempty"`
	TurnComplete     *TurnComplete     `json:"turn_complete,omitempty"`
	Error            *ErrorRecord      `json:"error,omitempty"`
	Usage            *UsageRecord      `json:"usage,omitempty"`
	Compact          *CompactRecord    `json:"compact,omitempty"`
	Subagent         *SubagentRecord   `json:"subagent,omitempty"`
}

func Replay(path string) ([]Record, error)
    Replay reads a session file and returns its records in file order.

func UnmarshalJSONLine(line []byte) (Record, error)

func (r Record) MarshalJSONLine() ([]byte, error)

type ReplayCache struct {
	// Has unexported fields.
}
    ReplayCache caches parsed records per file path. Entries are invalidated
    when the file's modtime changes. Capped via LRU eviction.

func NewReplayCache(capacity int) *ReplayCache
    NewReplayCache returns a cache with the given capacity (0 → 64 default).

func (c *ReplayCache) Clear()
    Clear empties the cache (used by /sessions search-rebuild-index).

func (c *ReplayCache) Get(path string) ([]Record, error)
    Get returns the cached records if present and the file's modtime hasn't
    changed; otherwise reads + parses the file, stores, evicts oldest if at cap.

func (c *ReplayCache) Invalidate(path string)
    Invalidate removes a single path from the cache (used by /sessions delete).

func (c *ReplayCache) Size() int
    Size returns the current entry count.

type SessionMeta struct {
	SessionID       string    `json:"session_id"`
	Cwd             string    `json:"cwd"`
	Model           string    `json:"model"`
	PermissionMode  string    `json:"permission_mode"`
	AnthrogoVersion string    `json:"anthrogo_version"`
	CreatedAt       time.Time `json:"created_at"`
}

type Store struct {
	// Has unexported fields.
}

func New(opts NewOptions) (*Store, error)

func NewSubagent(parent *Store, subagentID string) (*Store, error)
    NewSubagent creates a Store for a subagent invocation. The JSONL path is
    <parent.path-without-.jsonl>/subagents/<subagentID>.jsonl. The parent's
    directory and the subagents subdirectory are created if needed.

func Resume(cwd, sessionID string) (*Store, error)

func (s *Store) Append(r Record) error

func (s *Store) Close() error

func (s *Store) ID() string

func (s *Store) NewRecordHook() func(Record)

func (s *Store) Path() string

type SubagentRecord struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
}
    SubagentRecord is the payload for KindSubagentStart records emitted in the
    parent JSONL when RunSubagent begins. Replay treats it as informational only
    (no message added to history).

type ToolResult struct {
	ToolUseID string `json:"tool_use_id"`
	Text      string `json:"text"`
	IsError   bool   `json:"is_error,omitempty"`
}

type ToolUseRequest struct {
	ToolUseID string         `json:"tool_use_id"`
	ToolName  string         `json:"tool_name"`
	ToolInput map[string]any `json:"tool_input"`
}

type TurnComplete struct {
	StopReason string `json:"stop_reason"`
}

type UsageRecord struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens,omitempty"`
}

type UserMessage struct {
	Content []message.Block `json:"content"`
}

```

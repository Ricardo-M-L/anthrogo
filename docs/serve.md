# anthrogo serve — HTTP API Reference

`anthrogo serve` starts a long-lived HTTP daemon that exposes the anthrogo engine as a REST/SSE API. All endpoints return JSON. Streaming chat responses use Server-Sent Events (SSE).

## Starting the server

```bash
anthrogo serve --addr 127.0.0.1:8765
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | `127.0.0.1:8765` | TCP address to listen on |
| `--token` | _(none)_ | Optional Bearer auth token. When set, all routes require `Authorization: Bearer <token>` |
| `--cors-origin` | _(none)_ | Value for `Access-Control-Allow-Origin` response header |
| `--sessions-dir` | _(default ~/.anthrogo)_ | Override session storage root |
| `--model` | from settings.yaml | Model alias |
| `--provider` | from settings.yaml | Provider profile name |

---

## Authentication

When `--token` is set every request must include:

```
Authorization: Bearer <token>
```

Requests without or with a wrong token receive `401 Unauthorized`.

---

## Endpoints

### GET /v1/health

Returns server status.

```bash
curl http://127.0.0.1:8765/v1/health
```

Response:

```json
{
  "ok": true,
  "version": "0.13.13-dev",
  "uptime_seconds": 42.1,
  "in_flight_chats": 0
}
```

---

### POST /v1/chat

Send a message to a session. Sessions are lazily created on the first request.

#### Request body

```json
{
  "session_id": "my-session",
  "prompt": "What is the capital of France?",
  "stream": false
}
```

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `session_id` | string | yes | Identifies the conversation. Engines are cached per session (LRU, max 32). |
| `prompt` | string | yes | The user message to submit. |
| `stream` | bool | no | When `true`, response is SSE. When `false` (default), response is sync JSON. |

#### Sync response (`stream: false`)

```bash
curl -X POST http://127.0.0.1:8765/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","prompt":"Hello","stream":false}'
```

```json
{
  "session_id": "s1",
  "text": "Hello! How can I help you today?"
}
```

#### Streaming response (`stream: true`)

```bash
curl -X POST http://127.0.0.1:8765/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"session_id":"s1","prompt":"Write a haiku","stream":true}'
```

Each line is a standard SSE `data:` frame containing a JSON object:

```
data: {"type":"delta","text":"Autumn leaves fall slow"}

data: {"type":"delta","text":"\nSilence fills the morning air"}

data: {"type":"done"}
```

SSE event types:

| Type | Fields | Description |
|------|--------|-------------|
| `delta` | `text` | Partial assistant text |
| `tool_use` | `tool_name`, `tool_use_id` | Tool call started |
| `tool_result` | `tool_use_id` | Tool call result |
| `done` | — | Turn complete |
| `error` | `error` | Error occurred |

---

### GET /v1/sessions

List up to 100 sessions sorted by modification time (most recent first).

```bash
curl http://127.0.0.1:8765/v1/sessions
```

```json
[
  {"id": "550e8400-e29b-41d4-a716-446655440000", "mtime": "2026-05-21T14:00:00Z"},
  {"id": "7e2d4c8a-1f3b-4e5d-9a2c-123456789abc", "mtime": "2026-05-21T13:45:00Z"}
]
```

---

### GET /v1/sessions/{id}

Return full JSONL records for a session as a JSON array.

```bash
curl http://127.0.0.1:8765/v1/sessions/550e8400-e29b-41d4-a716-446655440000
```

Returns an array of session records (same schema as the `.jsonl` files under `~/.anthrogo/projects/`).

**Errors:** `404` when the session ID is not found.

---

### DELETE /v1/sessions/{id}

Delete a session file. Returns `204 No Content` on success.

```bash
curl -X DELETE http://127.0.0.1:8765/v1/sessions/550e8400-e29b-41d4-a716-446655440000
```

**Errors:** `404` when the session ID is not found.

---

### GET /v1/tools

List all registered tools with their names, descriptions, and input schemas.

```bash
curl http://127.0.0.1:8765/v1/tools
```

```json
[
  {
    "name": "Bash",
    "description": "Execute a bash command",
    "schema": {"type": "object", "properties": {"command": {"type": "string"}}}
  }
]
```

---

## Concurrency model

- One global `tool.Registry` and `permissions.Context` is shared across all sessions.
- Each session gets a lazily-created `*query.Engine`, cached in a `sync.RWMutex`-guarded map capped at **32 sessions**.
- When the cap is reached the **least-recently-used** session (oldest `lastAccess` timestamp) is evicted; its engine is discarded but its JSONL file on disk is unaffected.
- Each `/v1/chat` request is bound to the HTTP request's `context.Context`, so disconnects cancel the in-flight engine turn immediately.

---

## Authenticated example

```bash
anthrogo serve --addr 127.0.0.1:8765 --token mysecret &

curl -H 'Authorization: Bearer mysecret' \
     http://127.0.0.1:8765/v1/health
```

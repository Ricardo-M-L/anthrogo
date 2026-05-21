# anthrogo web — Browser UI

`anthrogo web` starts an HTTP server that serves an embedded single-page application
at `/` alongside the full REST/SSE API at `/v1/*`.

## Quick start

```bash
anthrogo web
# → anthrogo web listening on http://127.0.0.1:8766/
# → browser opens automatically
```

The browser opens automatically on macOS, Linux (xdg-open), and Windows.
Pass `--no-browser` to suppress this for headless or SSH environments.

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--addr` | auto (8766–8775) | Listen address. When omitted, the first free port in 8766–8775 is used. |
| `--no-browser` | false | Do not open the browser automatically. |
| `--token` | — | Optional Bearer auth token. Required by both the API and the UI. |
| `--cors-origin` | — | `Access-Control-Allow-Origin` header value. |
| `--sessions-dir` | — | Override session storage directory. |
| `--model` | settings.yaml | Model alias override. |
| `--provider` | settings.yaml | Provider profile override. |

## UI layout

- **Sessions panel** (left, 240 px): lists recent sessions by modification time. Click a session to load its history. The "+ New" button starts a fresh conversation.
- **Chat panel** (right): scrollable message history with role-coloured left borders (user = blue, assistant = green, tool = gray). Inline markdown rendering (bold, italic, code fences, links, headings).
- **Settings popover** (top right → "Settings" button): configure Bearer token, server base URL, and SSE streaming toggle. Settings are persisted in `localStorage`.
- **Theme toggle** (☀/🌙): switches between dark (default) and light themes.
- **Mobile**: below 768 px the sessions panel collapses into a drawer opened by the ☰ button.

## Bearer auth

When you start the server with `--token`:

```bash
anthrogo web --token mysecret --no-browser
```

Enter the same token in the Settings popover (the "Bearer Token" field). It is
stored in `localStorage` and sent as `Authorization: Bearer <token>` on every
request.

## Pointing the UI at a remote daemon

If you run `anthrogo serve` (or `anthrogo web --no-browser`) on a remote host,
you can point a locally-opened `index.html` at it:

1. Open `http://remote-host:8766/` in your browser (or use SSH port-forwarding).
2. Open Settings and set **Server URL** to `http://remote-host:8766`.
3. Set the Bearer token if the remote server uses `--token`.

The SPA uses `fetch` and the EventSource-compatible stream reader, so any
modern browser works. No build step is required — the assets are embedded in
the `anthrogo` binary via `embed.FS`.

## Endpoints served

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | index.html (SPA entry point) |
| GET | `/app.js` | SPA JavaScript |
| GET | `/styles.css` | SPA stylesheet |
| POST | `/v1/chat` | Send a message (sync JSON or SSE streaming) |
| GET | `/v1/sessions` | List recent sessions |
| GET | `/v1/sessions/{id}` | Fetch full JSONL records |
| DELETE | `/v1/sessions/{id}` | Delete a session |
| GET | `/v1/tools` | List registered tools |
| GET | `/v1/health` | Health + uptime |

See [docs/serve.md](serve.md) for full API reference and curl examples.

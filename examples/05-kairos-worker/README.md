# 05 — KAIROS worker

**What you'll learn:** run an anthrogo KAIROS worker, connect a client to it,
and dispatch a subagent task to the remote worker.

**Time:** ~15 minutes.

---

## What is KAIROS?

KAIROS is anthrogo's distributed subagent system. A *worker* node listens for
tasks over a gRPC/WebSocket channel; a *client* (your interactive session)
dispatches Task tool calls with `subagent_type: "remote:<name>"` and the worker
executes them independently.

Use cases:
- Offload heavy computation to a more powerful machine.
- Run multiple subagents in parallel without blocking your local session.
- Separate tool permissions (the worker can have broader access than the client).

---

## Prerequisites

| Item | How to get it |
|------|---------------|
| `anthrogo` binary | see [01-basic-chat](../01-basic-chat/) |
| `ANTHROPIC_API_KEY` | see [01-basic-chat](../01-basic-chat/) |
| Two terminals | or use tmux / a second shell session |
| `KAIROS_AUTH_TOKEN` | Any random string; must match in both processes |

---

## Architecture

```
Terminal A                          Terminal B
──────────────────────────────────  ──────────────────────────────────
anthrogo (client)                   anthrogo (worker)
  --config client-settings.yaml       --config worker-settings.yaml
  Receives user input                  --kairos-serve :9001
  Dispatches Task → remote           Listens on :9001
                  ──── gRPC ────>    Runs subagent
                  <─── result ────   Returns result
```

---

## Step 1 — Set environment variables

In **both** terminals:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
export KAIROS_AUTH_TOKEN=my-secret-token-123
```

---

## Step 2 — Start the worker (Terminal A)

```bash
cd examples/05-kairos-worker
ANTHROGO_HOME=$(pwd) anthrogo --config worker-settings.yaml --kairos-serve :9001
```

Expected output:

```
anthrogo 0.13.x  KAIROS worker
Listening on :9001
Ready.
```

The worker does not show an interactive prompt — it waits for tasks.

---

## Step 3 — Start the client (Terminal B)

```bash
cd examples/05-kairos-worker
ANTHROGO_HOME=$(pwd) anthrogo --config client-settings.yaml
```

Expected output:

```
anthrogo 0.13.x  (claude-sonnet-4-6 / anthropic)
KAIROS remotes: worker-local [connected]
Type /help for commands.

>
```

---

## Step 4 — Dispatch a subagent task

Ask the model to use the remote worker:

```
Using the remote worker, summarise what the number 42 is famous for.
```

The model will issue a Task tool call with `subagent_type: "remote:worker-local"`.
You will see the request forwarded to Terminal A, the worker executes it, and
the result returns to Terminal B.

---

## Step 5 — Verify in worker terminal

Terminal A should show something like:

```
[task] id=abc123 subagent=remote:worker-local
[task] id=abc123 completed in 3.2s
```

---

## Step 6 — Multi-hop (optional)

To allow the worker to itself dispatch further remote subagents, increase
`maxHops` in `client-settings.yaml` and restart. Read `docs/kairos.md` for the
full multi-hop topology guide.

---

## Settings reference

### worker-settings.yaml

| Key | Purpose |
|-----|---------|
| `kairos.role` | Must be `worker` |
| `kairos.listen` | Address:port to bind |
| `kairos.maxConcurrent` | Parallel subagents |
| `kairos.authToken` | Shared secret (env var ref) |

### client-settings.yaml

| Key | Purpose |
|-----|---------|
| `kairos.role` | Must be `client` |
| `kairos.remote[].name` | Identifier used in `subagent_type: "remote:<name>"` |
| `kairos.remote[].address` | Worker address |
| `kairos.remote[].authToken` | Must match worker token |
| `kairos.remote[].maxHops` | Max recursive dispatch depth |

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `KAIROS remotes: worker-local [disconnected]` | Ensure worker is running and firewall allows `:9001` |
| `auth failed` | Both sides must have the same `KAIROS_AUTH_TOKEN` |
| Worker exits immediately | Check `ANTHROPIC_API_KEY` is exported in Terminal A |

---

## Next steps

- **KAIROS reference** → `docs/kairos.md`
- **Subagents reference** → `docs/subagents.md`

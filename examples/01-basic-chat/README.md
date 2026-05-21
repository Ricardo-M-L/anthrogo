# 01 — Basic chat

**What you'll learn:** start anthrogo with a minimal config, ask a question,
inspect token usage and cost.

**Time:** ~5 minutes.

---

## Prerequisites

| Item | How to get it |
|------|---------------|
| `anthrogo` binary | `go install github.com/Ricardo-M-L/anthrogo/cmd/anthrogo@latest` or `make install` from the repo root |
| Anthropic API key | https://console.anthropic.com → API Keys |

---

## Step 1 — Export your API key

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

---

## Step 2 — Start anthrogo

Point `ANTHROGO_HOME` at this directory so anthrogo reads `settings.yaml` from
here instead of your global `~/.anthrogo/`.

```bash
cd examples/01-basic-chat
ANTHROGO_HOME=$(pwd) anthrogo
```

Expected startup output:

```
anthrogo 0.13.x  (claude-sonnet-4-6 / anthropic)
Type /help for commands.

>
```

---

## Step 3 — Ask a question

At the `>` prompt type:

```
What is the capital of France?
```

Expected response (abbreviated):

```
The capital of France is Paris.
```

---

## Step 4 — Inspect usage

```
/usage
```

Expected output (numbers will vary):

```
Session tokens — input: 42  output: 18  total: 60
```

---

## Step 5 — Inspect cost

```
/cost
```

Expected output:

```
Session cost — $0.0001
```

---

## Step 6 — Exit

```
/exit
```

---

## What's in settings.yaml

| Key | Value | Purpose |
|-----|-------|---------|
| `provider` | `anthropic` | Use the official Anthropic API |
| `model` | `claude-sonnet-4-6` | A capable mid-tier model |
| `mode` | `default` | Standard interactive chat |
| `apiKey` | `env:ANTHROPIC_API_KEY` | Read the key from an env var |

---

## Next steps

- **Add tools / hooks** → see [02-with-mcp](../02-with-mcp/)
- **Full config reference** → `docs/reference/yaml.md`

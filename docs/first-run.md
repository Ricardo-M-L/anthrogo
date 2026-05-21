# First run

This guide takes you from a fresh install to a working anthrogo session in about 5 minutes.

## Step 1 — Set your API key

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Or add it to `~/.anthrogo/settings.yaml` (created in step 2):

```yaml
apiKey: sk-ant-...
```

## Step 2 — Generate your settings file

```bash
anthrogo init-config
```

The interactive wizard asks for your API key, default model, and permission mode, then writes `~/.anthrogo/settings.yaml`. Use `--force` to overwrite an existing file.

## Step 3 — Verify your environment

```bash
anthrogo doctor
```

Runs ~20 checks and prints a structured PASS / WARN / FAIL report:

```
[✓] Go runtime                    running on go1.25.0 (darwin/arm64)
[✓] Settings file                 found at /Users/you/.anthrogo/settings.yaml
[✓] API key: ANTHROPIC_API_KEY    set (anthropic)
[✓] Anthrogo home                 /Users/you/.anthrogo ok
[!] Binary: docker                not on PATH (ContainerExec tool)
[!] Binary: whisper               not on PATH (SpeechToText tool)
[✓] Anthropic API                 HTTP 200
[✓] anthrogo version              0.13.0-dev

Summary: 7 PASS, 2 WARN, 0 FAIL
```

Exit code is `1` when any check is FAIL, `0` otherwise. `WARN` means an optional feature is unavailable, not that the core CLI is broken.

## Step 4 — Launch the TUI

```bash
anthrogo
```

The Bubble Tea TUI opens. Type a prompt and press Enter. Press **F2** to cycle pane layouts (single / split / triple). Press **Up/Down** to scroll through input history.

## Step 5 — Try compaction

After a long session, run `/compact` to summarize older turns and reduce token cost:

```
/compact
```

Or set `auto_compact_threshold: 150000` in `settings.yaml` to trigger automatically.

## Headless mode

For scripting, skip the TUI:

```bash
anthrogo -p "explain main.go"
anthrogo --permission-mode acceptEdits -p "fix the typo in README"
git diff | anthrogo -p "summarize this diff"
echo "what is 2+2?" | anthrogo -p
```

The `--json` flag emits one JSON object per line (ndjson) — useful for piping to `jq`:

```bash
cat README.md | anthrogo --json -p "describe in 3 bullets" | jq -r '.text // empty'
```

## Useful first commands

Once in the TUI:

| Command | Effect |
|---------|--------|
| `/help` | List all slash commands |
| `/cost` | Show token usage and estimated cost |
| `/compact` | Summarize older turns |
| `/sessions` | List saved sessions |
| `/version` | Print version + check for updates |
| `/theme set light` | Switch to light theme |
| `/mcp` | List connected MCP servers |

# Slash commands

Slash commands are typed directly in the TUI prompt. Press Tab to autocomplete.

## Core

| Command | Description |
|---------|-------------|
| `/help` | List available commands |
| `/quit` | Exit the session |
| `/version` | Print version + check GitHub for a newer release |
| `/version no-check` | Print version only, no network call |

## Session

| Command | Description |
|---------|-------------|
| `/sessions` / `/sessions list` | List sessions for the current cwd |
| `/sessions show <id>` | Metadata summary for a session |
| `/sessions replay <id>` | Render session as a timeline |
| `/sessions search <q>` | Search session JSONLs (flags: `--regex`, `--recurse-subagents`, `--since`, `--until`) |
| `/sessions delete <id>` | Dry-run delete; add `--yes` to confirm |
| `/sessions export <id> [-o file.md]` | Export session as Markdown |
| `/sessions stats [--since] [--until]` | Aggregate token and cost metrics |
| `/sessions diff <id1> <id2>` | LCS diff of two sessions |
| `/sessions reindex` | Clear in-memory LRU search cache |
| `/audit [list N]` | Scan sessions for tool calls / errors |
| `/audit by-tool <name>` | Filter audit to a specific tool |
| `/audit errors` | Show only error records |
| `/audit search <q>` | Search audit records |

## Cost and compaction

| Command | Description |
|---------|-------------|
| `/cost` | Session token usage + estimated cost |
| `/cost reset` | Zero the cumulative cost counter |
| `/usage` | Detailed usage + auto-compact state |
| `/compact [--keep N] [--reset-budget]` | Summarize older turns |

## Input history

| Command | Description |
|---------|-------------|
| `/history` / `/history list [N]` | Show the N most recent prompts (default 20) |
| `/history search <q>` | Case-insensitive substring search |
| `/history clear` | Delete the history file |

## Configuration

| Command | Description |
|---------|-------------|
| `/theme list` | Show available themes |
| `/theme show` | Print current theme name |
| `/theme set <name>` | Switch theme immediately |
| `/mode <mode>` | Switch permission mode: `default` / `acceptEdits` / `plan` / `bypassPermissions` |
| `/system show` | Print active system prompt + overlay files |
| `/system edit [home\|project]` | Open `$EDITOR` on an overlay file inline |
| `/system reset [home\|project]` | Remove an overlay file |
| `/telemetry status` | Show telemetry opt-in state |

## MCP

| Command | Description |
|---------|-------------|
| `/mcp` | List all servers and status |
| `/mcp status <name>` | One server's last error + redacted headers |
| `/mcp reload` | Remove and re-register all `mcp__*` tools |

## Plugins / skills / subagents

| Command | Description |
|---------|-------------|
| `/plugin` | List installed plugins |
| `/plugin info <name>` | Show manifest |
| `/plugin install <src>` | Install from local path, HTTPS archive, or git URL |
| `/plugin remove <name>` | Remove a plugin |
| `/plugin reload` | Hot-reload plugins |
| `/skills` | List loaded skills |
| `/skills show <name>` | Print a skill's SKILL.md |
| `/skills reload` | Re-scan skill directories |
| `/subagents` | List loaded subagent types |
| `/subagents show <name>` | Inspect a type definition |
| `/subagents reload` | Hot-reload subagent YAML files |

## Login

| Command | Description |
|---------|-------------|
| `/login` | OAuth 2.1 PKCE flow; saves token to `~/.anthrogo/auth/anthropic.json` |
| `/login status` | Show current token expiry |
| `/login logout` | Remove cached token |

# 02 — With MCP

**What you'll learn:** register an MCP filesystem server, inspect its tools
with `/mcp list`, call a tool via natural language, and reload after a config
change.

**Time:** ~10 minutes.

---

## Prerequisites

| Item | How to get it |
|------|---------------|
| `anthrogo` binary | see [01-basic-chat](../01-basic-chat/) |
| `ANTHROPIC_API_KEY` | see [01-basic-chat](../01-basic-chat/) |
| Node.js + npm | https://nodejs.org — required to run `npx @modelcontextprotocol/server-filesystem` |

Verify Node is installed:

```bash
node --version   # should print v18+ or v20+
npx --version
```

---

## Step 1 — Start anthrogo

```bash
cd examples/02-with-mcp
ANTHROGO_HOME=$(pwd) anthrogo
```

anthrogo will launch the `filesystem` MCP server in the background before
showing the prompt. You should see:

```
anthrogo 0.13.x  (claude-sonnet-4-6 / anthropic)
MCP: filesystem [running]
Type /help for commands.

>
```

If you see `MCP: filesystem [error]`, see the troubleshooting section in
[notes.md](notes.md).

---

## Step 2 — List MCP tools

```
/mcp list
```

Expected output (tool names depend on server version):

```
MCP servers (1 connected)

  filesystem  [running]
    Tools:
      read_file
      write_file
      list_directory
      ...
```

---

## Step 3 — Call a tool

Ask the model to use the filesystem server:

```
List all files in /tmp and tell me how many there are.
```

The model will:
1. Call `filesystem.list_directory` with path `/tmp`
2. Count the results
3. Report back to you

---

## Step 4 — Reload after a config edit

Edit `settings.yaml` to add a second allowed path, then reload without
restarting anthrogo:

```
/mcp reload
```

---

## Step 5 — Check server status

```
/mcp status
```

Expected output:

```
  filesystem  pid=12345  [running]
```

---

## What's in settings.yaml

| Key | Meaning |
|-----|---------|
| `mcpServers.filesystem.command` | Executable to run (`npx`) |
| `mcpServers.filesystem.args` | Arguments passed to the command |

See [notes.md](notes.md) for full slash-command reference.

---

## Next steps

- **Write a custom skill** → [03-custom-skill](../03-custom-skill/)
- **MCP reference** → `docs/mcp.md`

# MCP slash-command reference

These commands are available once anthrogo has connected to at least one MCP server.

## /mcp list

Lists all registered MCP servers and the tools each exposes.

```
/mcp list
```

Example output:

```
MCP servers (1 connected)

  filesystem  [running]
    Tools:
      read_file       Read the contents of a file at a path.
      write_file      Write content to a file at a path.
      list_directory  List the contents of a directory.
      create_directory  Create a new directory.
      delete_file     Delete a file at a path.
      move_file       Move or rename a file.
      search_files    Recursively search for files matching a pattern.
      get_file_info   Retrieve metadata for a file or directory.
```

---

## /mcp status

Shows the running state and PID of each MCP server process.

```
/mcp status
```

---

## /mcp reload

Restarts all MCP servers and re-registers their tools with the model.
Useful after editing `mcpServers` in settings.yaml without restarting anthrogo.

```
/mcp reload
```

---

## Asking the model to use an MCP tool

Once connected, you can ask the model naturally:

```
List all files in /tmp
```

The model will call `filesystem.list_directory` and show you the results.

```
Write "hello world" to /tmp/test.txt
```

The model will call `filesystem.write_file`.

---

## Troubleshooting

| Symptom | Fix |
|---------|-----|
| `npx` not found | Install Node.js (https://nodejs.org) |
| Server fails to start | Run the `npx` command manually to see the error |
| Tools missing after reload | Check stderr in the anthrogo log with `ANTHROGO_DEBUG=1` |

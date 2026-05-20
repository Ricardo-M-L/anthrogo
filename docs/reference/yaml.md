# Settings YAML reference

See [README — Configuration](https://github.com/Ricardo-M-L/anthrogo#configuration).

Default location: `~/.anthrogo/settings.yaml`

## Top-level keys

```yaml
# Provider selection
provider: anthropic          # anthropic | openai-compat | bedrock | vertex | failover
model: claude-opus-4-5

# Compaction
compaction:
  auto: true
  threshold: 0.85

# Cost / budget
cost:
  budget_usd: 5.00

# MCP servers
mcp:
  servers:
    - name: my-server
      transport: stdio
      command: ["./mcp-server"]

# Hooks
hooks:
  PreToolUse:
    - command: ["./hooks/pre-tool"]
  PostToolUse:
    - command: ["./hooks/post-tool"]

# KAIROS
kairos:
  nodes:
    - address: 10.0.0.10:9090
      name: remote-node
```

(Full YAML reference migrating from README — M11.4 follow-up.)

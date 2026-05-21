# anthrogo examples

5 minimal projects demonstrating common patterns. Each is self-contained — set
`ANTHROGO_HOME` to its directory and run anthrogo from there.

| # | Directory | Demonstrates |
|---|-----------|--------------|
| 01 | [basic-chat](01-basic-chat/) | First-run flow, /usage, /cost |
| 02 | [with-mcp](02-with-mcp/) | MCP filesystem server integration |
| 03 | [custom-skill](03-custom-skill/) | Writing a SKILL.md, invoking via Task |
| 04 | [plugin-bundle](04-plugin-bundle/) | Plugin manifest + commands + skills + hooks |
| 05 | [kairos-worker](05-kairos-worker/) | KAIROS worker + client subagent |

## Quick start

```bash
cd examples/01-basic-chat
ANTHROGO_HOME=$(pwd) anthrogo
```

Each example's `README.md` has full step-by-step instructions and expected output.

## Prerequisites

- `anthrogo` installed and on your `$PATH` (see [docs/install.md](../docs/install.md))
- `ANTHROPIC_API_KEY` exported in your shell (examples 01-04)
- Examples 02 and 05 have additional requirements listed in their READMEs

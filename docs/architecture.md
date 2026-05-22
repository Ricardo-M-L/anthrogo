# Architecture

This page sketches anthrogo's runtime topology and how the major packages
fit together. For per-package API details see the [API reference](api/).

## Top-level: one binary, four entry modes

```mermaid
flowchart LR
    user([User])

    subgraph cmd[cmd/anthrogo]
        cli[anthrogo CLI]
    end

    user -->|terminal| cli

    cli -->|default| tui[TUI mode<br/>Bubble Tea]
    cli -->|"-p / --json"| headless[Headless mode]
    cli -->|"serve"| serve[HTTP daemon<br/>internal/serve]
    cli -->|"web"| web[Browser UI<br/>internal/web]

    tui --> engine
    headless --> engine
    serve --> engine
    web --> serve

    subgraph engine[pkg/query · Engine]
        loop[Turn loop]
        subreg[Subagent registry]
        toolreg[Tool registry]
        permgate[Permission gate]
        hooks[Hook manager]
    end

    engine --> provider
    engine --> mcp[(internal/mcp<br/>MCP servers)]
    engine --> session[(internal/session<br/>JSONL + SQLite L2)]
    engine --> tools

    subgraph provider[pkg/provider]
        anthropic[Anthropic]
        openai[OpenAI-compat]
        bedrock[Bedrock]
        vertex[Vertex]
        ollama[Ollama]
        failover[Failover]
    end

    subgraph tools[pkg/tool<br/>30+ built-ins]
        readtool[Read/Write/Edit]
        bashtool[Bash sandbox]
        httptool[HTTPRequest +<br/>WebFetch +<br/>WebSearch]
        netguard{{NetGuard<br/>SSRF block}}
        browser[BrowserAction<br/>chromedp]
        sql[SQLQuery]
        pdfxlsx[PDFRead +<br/>XlsxRead]
        commsl[SlackPost +<br/>Embed + ImageGen +<br/>CalendarEvent]
        readtool & httptool & browser & sql & pdfxlsx & commsl
        httptool -.-> netguard
    end
```

## Engine turn loop

```mermaid
sequenceDiagram
    autonumber
    participant Caller as Caller<br/>(TUI / serve / headless)
    participant Engine as pkg/query · Engine
    participant Provider as Provider
    participant Gate as pkg/permissions
    participant Tools as pkg/tool
    participant Hooks as internal/hooks

    Caller->>Engine: SubmitMessage(ctx, text)
    Engine->>Provider: Stream(req)

    loop until EventMessageStop
        Provider-->>Engine: EventTextDelta / ToolUseStart / ...
        Engine-->>Caller: forward as Event channel
    end

    alt stop_reason == tool_use
        par concurrent dispatch (when all safe)
            Engine->>Gate: Decide(ctx, c, tool, input)
            Gate-->>Engine: Allow / Ask / Deny
            Engine->>Hooks: FirePreToolUse(ctx, tool, input)
            Hooks-->>Engine: HookDecision
            Engine->>Tools: t.Call(ctx, input, ctx)
            Tools-->>Engine: Result
            Engine->>Hooks: FirePostToolUse(ctx, tool, in, out)
        end
        Engine->>Engine: append tool_results
        Engine->>Provider: Stream(req with results)
    end

    Engine-->>Caller: Done
```

## Subagent dispatch (M5.1+)

```mermaid
flowchart TB
    parent[Parent Engine]
    parentctx[Parent Tool ctx<br/>permissions + hooks + sessions]

    parent -->|"Task tool"| dispatch[Engine.RunSubagent]
    dispatch --> child[Child Engine]

    parentctx -.->|Cloned<br/>permissions.Context.Clone| childctx
    parentctx -.->|Shared by ref<br/>hooks.Manager| childctx[Child Tool ctx]

    child --> childloop[Child Turn Loop]
    childloop -->|"final text"| dispatch
    dispatch -->|"tool_result"| parent
```

For KAIROS (cross-process subagent dispatch) see [docs/kairos.md](kairos.md).

## Session storage

```mermaid
flowchart LR
    engine[Engine] -->|"records"| store[internal/session<br/>Store]
    store -->|"flock LOCK_EX"| jsonl[(<cwd>/.anthrogo/<id>.jsonl<br/>mode 0o600)]
    store -->|"Flush each record<br/>Sync on TurnComplete/Error/Compact"| jsonl

    cli[anthrogo --resume<br/>anthrogo /sessions replay] --> replay[Replay]
    replay --> jsonl
    replay -->|"L1 miss"| persistent[PersistentCache<br/>SQLite WAL]
    persistent -->|"<= 2000 rows<br/>LRU trim on insert"| db[(~/.anthrogo/cache.db)]
```

## Hook flow

```mermaid
flowchart LR
    engine[Engine] -->|"PreToolUse(ctx)"| runner[hooks.Runner]
    runner --> exec[exec.CommandContext]
    exec -->|"stdin: JSON payload"| script[(User-supplied script)]
    script -->|"stdout: JSON Output"| runner

    runner -.->|"strip ANTHROPIC_/<br/>OPENAI_/AWS_/<br/>GITHUB_TOKEN..."| sanitized[sanitizedEnv]
    sanitized -.-> exec
```

## MCP topology

```mermaid
flowchart LR
    subgraph anthrogo
        mgr[internal/mcp.Manager]
        toolreg[(Tool Registry)]
    end

    mgr -->|"stdio"| s1[(MCP subprocess<br/>e.g. context7)]
    mgr -->|"SSE / HTTP"| s2[(Remote MCP server)]
    mgr -->|"Streamable HTTP +<br/>OAuth 2.1 PKCE"| s3[(Enterprise MCP)]
    mgr -->|"WebSocket"| s4[(Realtime MCP)]

    s1 & s2 & s3 & s4 -->|"tools/list"| mgr
    mgr -->|"register as mcp__name__tool"| toolreg
```

## Permission gate

The gate is the single point of decision for every tool invocation. Order
documented in `pkg/permissions/gate.go::Decide`:

1. `ModeBypassPermissions` + `IsBypassAvailable` → allow always.
2. Deny rules win unconditionally.
3. `ModeAcceptEdits` auto-allows `Write` / `Edit` / `NotebookEdit`.
4. `HookDecide(ctx, tool, input)` may short-circuit Allow / Deny / Pass.
5. Allow rules → allow (plan mode still locks write tools).
6. Ask rules → ask (or deny if `ShouldAvoidPrompts`).
7. Fallback → ask in default mode, deny in `ShouldAvoidPrompts`.

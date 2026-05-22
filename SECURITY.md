# Security Policy

## Supported versions

anthrogo is pre-1.0; only the latest tagged version receives security fixes.
The current development line is `0.13.x-dev`. See the [CHANGELOG](CHANGELOG.md)
for the most recent release.

## Reporting a vulnerability

**Do not open a public GitHub issue for a suspected vulnerability.**

Email the maintainer privately with:

- A short description of the issue (one paragraph is enough).
- Reproduction steps or a minimal proof-of-concept.
- Your assessment of impact (data loss, code execution, credential exposure,
  etc.) and any affected components (CLI, MCP server interaction, hooks,
  serve daemon, web UI, a specific tool).

You can expect:

- An acknowledgement within 3 business days.
- A coordinated disclosure timeline — typically a fix in the next dev release
  followed by public disclosure 30-90 days later, depending on severity and
  whether downstream users need lead time to update.

If the issue is reachable only via a specific provider, MCP server, or hook
configuration, please include that context — anthrogo is a host process that
runs user-configured subprocesses, so the trust boundary differs from a
typical web service.

## What's in scope

- **anthrogo source** in this repository: the CLI binary, all packages under
  `pkg/`, `internal/`, and `cmd/anthrogo/`.
- **The HTTP daemon (`anthrogo serve`)** and **web UI (`anthrogo web`)** — both
  bind to `127.0.0.1` by default but operators sometimes expose them.
- **Built-in tools** under `pkg/tool/` — especially those that perform IO
  (Bash, Write, Edit, HTTPRequest, WebFetch, Slack, etc.).
- **Hook execution** in `internal/hooks/` and the pre-tool-use gate in
  `pkg/permissions/`.

## What's out of scope

- **Third-party LLM providers** (Anthropic, OpenAI-compatible endpoints,
  Bedrock, Vertex, Ollama). Report directly to the provider.
- **User-supplied MCP servers**. anthrogo runs whatever the user configures;
  trust is a user responsibility for each `mcpServers:` entry.
- **User-supplied hook scripts**. Hooks run with the user's UID. anthrogo
  strips credential-bearing env vars (see [V5 in
  CHANGELOG](CHANGELOG.md#v5)), but the trust model is still
  user-determines-which-scripts-to-install.
- **Tool calls authorized by the gate**. If the user (or a configured
  allow-rule) approved a destructive Bash command, anthrogo ran it as
  requested.

## Past audits

Two internal audits ran post-feature-freeze (M14, M15). Each report ID
(V*, R*, K*, D*, O*, T*) maps to a commit in `git log`. The full list of
shipped fixes is in the CHANGELOG under the `0.13.15-dev` through
`0.13.21-dev` entries.

A summary of the threat model these audits covered:

- SSRF via LLM-supplied URLs (httprequest, webfetch, embed, imagegen) — fixed
  V2.
- Path traversal in session IDs — fixed S2/V2.
- Privilege escalation via archive setuid/setgid bits — fixed V4.
- Credential leakage via hook subprocess env — fixed V5.
- TLS-bypass via subagent YAML — fixed K3.
- TOCTOU in serve session cache — fixed S3/V3.
- XSS in the web UI — fixed S1/V1.

## Coordinated disclosure

If a third party reports a vulnerability that affects an upstream dependency
(chromedp, the Anthropic SDK, modernc/sqlite, etc.), we will route the report
upstream and track the fix. We will not publicly disclose the issue until
the upstream patch is released.

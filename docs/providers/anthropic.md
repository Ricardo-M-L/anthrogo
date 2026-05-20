# Anthropic provider

See [README — Providers](https://github.com/Ricardo-M-L/anthrogo#providers).

The Anthropic provider connects directly to `api.anthropic.com` using the official Messages API.

## Configuration

```yaml
provider: anthropic
model: claude-opus-4-5
```

Set your API key via environment variable:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

## Supported features

- Streaming responses
- Tool use
- Extended thinking (claude-3-7-sonnet and above)
- Prompt caching (automatic on long contexts)
- count_tokens API for accurate token counting

(Full Anthropic provider reference migrating from README — M11.4 follow-up.)

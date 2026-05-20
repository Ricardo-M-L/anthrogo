# Compaction

See [README — Compaction](https://github.com/Ricardo-M-L/anthrogo#compaction).

Compaction reduces context window usage by summarizing old conversation turns while preserving the working context.

## Manual compaction

```
/compact
```

Summarizes all turns older than the current task boundary into a concise context block.

## Auto-compaction

anthrogo monitors token usage and triggers `/compact` automatically when the context approaches the model's limit. The threshold is configurable in `settings.yaml`:

```yaml
compaction:
  auto: true
  threshold: 0.85   # compact when context is 85% full
```

(Full compaction reference migrating from README — M11.4 follow-up.)

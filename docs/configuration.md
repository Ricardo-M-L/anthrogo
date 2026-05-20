# Configuration

See [README — Configuration](https://github.com/Ricardo-M-L/anthrogo#configuration).

## Config file

anthrogo reads `~/.anthrogo/settings.yaml` on startup. A minimal config:

```yaml
provider: anthropic
model: claude-opus-4-5
```

See the [Settings YAML reference](reference/yaml.md) for all available keys.

## Environment variables

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Anthropic API key |
| `ANTHROGO_CONFIG` | Override config file path |

(Full configuration reference migrating from README — M11.4 follow-up.)

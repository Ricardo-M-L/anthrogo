# OpenAI-compatible provider

See [README — Providers](https://github.com/Ricardo-M-L/anthrogo#providers).

anthrogo supports any OpenAI-compatible API endpoint, including DeepSeek, Kimi, MiniMax, and GLM.

## Configuration

```yaml
provider: openai-compat
base_url: https://api.deepseek.com/v1
model: deepseek-chat
```

Set credentials:

```bash
export OPENAI_COMPAT_API_KEY=your-key
```

## Known providers tested

| Provider | base_url |
|---|---|
| DeepSeek | `https://api.deepseek.com/v1` |
| Kimi | `https://api.moonshot.cn/v1` |
| MiniMax | `https://api.minimaxi.com/anthropic` |
| GLM | `https://open.bigmodel.cn/api/paas/v4` |

(Full OpenAI-compat provider reference migrating from README — M11.4 follow-up.)

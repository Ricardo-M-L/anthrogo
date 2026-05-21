# Provider failover

`providers_failover` lists profiles to try in order if the active provider emits an error **before** any text, tool-use, or usage event has been streamed.

## How it works

Once a "committed" event has been forwarded to the client, the error passes through unchanged — partial streams cannot be retried transparently. Only pre-commit errors trigger failover.

## Configuration

```yaml
provider: anthropic
providers_failover: [deepseek, kimi]
# On EventError before text/tool/usage from anthropic, retries with deepseek.
# If deepseek also fails (pre-commit), falls back to kimi.

profiles:
  deepseek:
    type: openai
    base_url: https://api.deepseek.com
    model: deepseek-chat
    api_key: env:DEEPSEEK_API_KEY
  kimi:
    type: openai
    base_url: https://api.moonshot.cn/v1
    model: kimi-k2-0905-preview
    api_key: env:KIMI_API_KEY
```

## Known limitations

- No backoff between attempts
- No selective retry by HTTP status code
- Partial-stream retry requires buffering (deferred)

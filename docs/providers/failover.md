# Provider failover

See [README — Providers](https://github.com/Ricardo-M-L/anthrogo#providers).

anthrogo supports automatic failover across multiple providers when a request fails (rate limit, timeout, or error).

## Configuration

```yaml
provider: failover
providers:
  - name: primary
    type: anthropic
  - name: fallback
    type: openai-compat
    base_url: https://api.deepseek.com/v1
    model: deepseek-chat
```

When the primary provider returns a retryable error, anthrogo automatically retries on the next provider in the list.

## Retry policy

```yaml
failover:
  max_retries: 3
  backoff: exponential
```

(Full failover reference migrating from README — M11.4 follow-up.)

# Cost tracking

anthrogo ships built-in pricing defaults for major models and tracks token usage and estimated cost for every session.

## /cost command

```
/cost          # print full usage summary
/cost reset    # zero the cumulative cost counter
```

Sample output:

```
Session usage: 12345 input + 1234 output = 13579 total tokens
Estimated cost: $0.0555 USD
```

The TUI status line shows the running cost (`$0.0234`) live during the session.

## Built-in pricing defaults

Built-in rates are sourced from published pricing as of 2026-05. They cover:

- **Claude**: opus-4-7 / sonnet-4-6 / haiku-4-5
- **OpenAI**: gpt-5* / gpt-4o / gpt-4o-mini / gpt-4-turbo / o1* / o3*
- **DeepSeek**: chat / reasoner
- **Kimi**: k2*
- **MiniMax**: M2 / abab*
- **GLM**: 4* / zero-*
- **Ollama-hosted models**: llama3* / llama-* / qwen2.5-* / qwen3* / mistral* / codellama* / phi* / gemma* — all $0/M (no per-token cost for local inference)

Rates will drift until the next anthrogo release updates them.

## Custom pricing

Add a `pricing:` stanza in `~/.anthrogo/settings.yaml` to override built-ins or add unlisted models:

```yaml
pricing:
  claude-sonnet-4-6:
    input_per_m: 3.0
    output_per_m: 15.0
  claude-haiku-4-5-*:
    input_per_m: 1.0
    output_per_m: 5.0
  deepseek-chat:
    input_per_m: 0.27
    output_per_m: 1.1
  "anthropic.claude-sonnet-4-6-v1:0":  # Bedrock model ID
    input_per_m: 3.0
    output_per_m: 15.0
```

Keys are exact model names or glob patterns (`filepath.Match` syntax; `*` matches within a path segment). Rates are USD per one million tokens. User-supplied keys always win over built-in defaults.

## Budget cap

To hard-cap spending, add `cost_limit_usd` to `settings.yaml` or pass `--cost-limit` on the command line:

```yaml
cost_limit_usd: 0.50   # deny tools after ~$0.50 of estimated spend
```

```bash
anthrogo --cost-limit 0.50
```

Once the cumulative estimated session cost reaches the limit, all tool calls are denied with a message showing the current cost and the limit. Set `--cost-limit 0` (or omit the field) to disable.

To reset the counter (e.g. after `/compact`) without ending the session:

```
/cost reset
```

Or automatically when compacting:

```
/compact --reset-budget
```

The budget cap remains armed after reset; usage accumulates again from zero.

## Token counting

anthrogo uses a real BPE tokenizer (tiktoken-go) for OpenAI-family models and a char/4 approximation for Claude and other models. Image tokens are not counted client-side; the provider's `EventUsage` is authoritative for image cost.

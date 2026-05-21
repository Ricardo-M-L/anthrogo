# OpenAI-compatible provider

anthrogo supports any OpenAI Chat Completions-compatible endpoint via profile `type: openai`. This covers DeepSeek, Kimi, MiniMax, GLM, vllm, and many others.

## Configuration via profiles

```yaml
provider: deepseek           # set active profile
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
  minimax:
    type: openai
    base_url: https://api.minimaxi.com/v1
    model: MiniMax-M2
    api_key: env:MINIMAX_API_KEY
  glm:
    type: openai
    base_url: https://open.bigmodel.cn/api/paas/v4
    model: glm-4.6
    api_key: env:GLM_API_KEY
```

## Known providers tested

| Provider | type | base_url | Model examples |
|----------|------|----------|----------------|
| DeepSeek | openai | `https://api.deepseek.com` | `deepseek-chat`, `deepseek-reasoner` |
| Kimi | openai | `https://api.moonshot.cn/v1` | `kimi-k2-0905-preview` |
| MiniMax | openai | `https://api.minimaxi.com/v1` | `MiniMax-M2`, `abab6.5s-chat` |
| GLM | openai | `https://open.bigmodel.cn/api/paas/v4` | `glm-4.6`, `glm-zero-preview` |
| vllm | openai | `http://localhost:8000/v1` | any loaded model |

## Switching profiles at runtime

```bash
anthrogo --provider kimi
anthrogo --provider deepseek -p "summarize this repo"
```

## Built-in pricing defaults

| Model | Input ($/M) | Output ($/M) |
|-------|-------------|--------------|
| deepseek-chat | 0.27 | 1.10 |
| deepseek-reasoner | — | — |
| kimi-k2* | — | — |
| MiniMax-M2 | — | — |
| glm-4* / glm-zero-* | — | — |

Add explicit `pricing:` entries for unlisted models or negotiated rates.

## Vision support

OpenAI-compatible image blocks are converted to `{type: "image_url", image_url: {url: "data:<mime>;base64,..."}}`  multimodal content arrays. Use `@image:<path>` in your prompt.

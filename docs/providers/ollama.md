# Ollama provider

Profile `type: ollama` is a thin convenience wrapper around the OpenAI-compatible endpoint that Ollama exposes at `http://localhost:11434/v1/chat/completions`. No real API key is required.

Added in M12.5.

## Requirements

Install and start Ollama, then pull a model:

```bash
# Install Ollama — see https://ollama.com/download
ollama pull llama3
ollama pull qwen2.5-coder
```

## Configuration

```yaml
provider: ollama-llama3
profiles:
  ollama-llama3:
    type: ollama
    model: llama3         # any model installed with `ollama pull`
  ollama-qwen:
    type: ollama
    base_url: http://localhost:11434  # explicit; this is also the default
    model: qwen2.5-coder
```

## How it works

`type: ollama` reuses the M7.1 OpenAI provider under the hood. It:
1. Sets `base_url` to `http://localhost:11434` if not provided
2. Uses the sentinel API key `"ollama"` (Ollama ignores it)
3. Routes to `/v1/chat/completions` (the OpenAI-compat shim, not Ollama's native `/api/chat`)

## Built-in pricing defaults

Common Ollama-hosted models have `$0/M` pricing entries built in (no per-token charges for local inference):

| Pattern | Cost |
|---------|------|
| `llama3*`, `llama-*` | $0/M |
| `qwen2.5-*`, `qwen3*` | $0/M |
| `mistral*` | $0/M |
| `codellama*` | $0/M |
| `phi*` | $0/M |
| `gemma*` | $0/M |

## Switching profiles

```bash
anthrogo --provider ollama-llama3
anthrogo --provider ollama-qwen -p "review main.go"
```

## Known limitations

- **Tool/function calling**: not all Ollama models support tool calling reliably. Test with your model before enabling tool-heavy workflows. Models that do not support tools will return errors on tool-use requests.
- **Thinking output**: Ollama "thinking" content (e.g., qwen3 `<think>` tags) flows through as regular text — no special thinking-block handling.
- **Native API**: only the OpenAI-compat shim (`/v1/chat/completions`) is used. The native Ollama `/api/chat` endpoint is not supported.
- **Remote Ollama**: set `base_url` to a non-localhost address to point at a remote Ollama instance. Ensure the instance is accessible and authentication is handled at the network layer.

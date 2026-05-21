# Anthropic provider

The Anthropic provider connects directly to `api.anthropic.com` using the official Messages API with streaming.

## Configuration

```yaml
provider: anthropic
model: claude-sonnet-4-6
```

## Authentication

**Environment variable (recommended):**

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

**Settings file:**

```yaml
apiKey: sk-ant-...
```

**OAuth 2.1 PKCE** (M11.5) — for corporate SSO / self-hosted IdPs (Auth0, Okta, Entra ID, Keycloak):

```yaml
auth:
  authorization_url: https://your-idp.example.com/oauth2/authorize
  token_url:         https://your-idp.example.com/oauth2/token
  client_id:         your-client-id
  scopes: [openid, profile]
  redirect_port: 8765
```

Then run `/login` in the TUI. When a valid token is present it is used automatically; `ANTHROPIC_API_KEY` / `apiKey` remain as the fallback.

```
/login          # opens browser, saves token to ~/.anthrogo/auth/anthropic.json
/login status   # show current token expiry
/login logout   # remove cached token
```

## Supported models

Use any model ID from the Anthropic model list. Pass with `--model` or set in `settings.yaml`:

- `claude-opus-4-7` — most capable
- `claude-sonnet-4-6` — balanced (default)
- `claude-haiku-4-5-20251001` — fastest

## Built-in pricing defaults

| Model | Input ($/M) | Output ($/M) |
|-------|-------------|--------------|
| claude-sonnet-4-6 | 3.0 | 15.0 |
| claude-haiku-4-5* | 1.0 | 5.0 |

Override in `settings.yaml`:

```yaml
pricing:
  claude-sonnet-4-6:
    input_per_m: 3.0
    output_per_m: 15.0
```

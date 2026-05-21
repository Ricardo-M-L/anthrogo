# Google Vertex AI provider

Profile `type: vertex` routes to Claude models hosted on Google Cloud Vertex AI. No `ANTHROPIC_API_KEY` is needed.

## Configuration

Both `region` and `project_id` are mandatory:

```yaml
profiles:
  vertex-sonnet:
    type: vertex
    model: claude-sonnet-4-6@20260101
    region: us-east5
    project_id: my-gcp-project
```

Activate:

```yaml
provider: vertex-sonnet
```

## Authentication

Uses Google Application Default Credentials — no `api_key` field:

```bash
# Local development
gcloud auth application-default login

# Service account
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

Workload identity on GKE / Cloud Run works automatically.

## Model IDs

Vertex model IDs follow the Model Garden convention and differ from direct Anthropic API names:

| Anthropic name | Vertex model ID |
|----------------|----------------|
| claude-sonnet-4-6 | `claude-sonnet-4-6@20260101` |

## Pricing

Add explicit pricing entries for Vertex model IDs:

```yaml
pricing:
  "claude-sonnet-4-6@20260101":
    input_per_m: 3.0
    output_per_m: 15.0
```

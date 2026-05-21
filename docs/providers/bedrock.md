# AWS Bedrock provider

Profile `type: bedrock` routes to Claude models via AWS Bedrock. No `ANTHROPIC_API_KEY` is needed.

## Configuration

```yaml
profiles:
  bedrock-sonnet:
    type: bedrock
    model: anthropic.claude-sonnet-4-6-v1:0
    region: us-west-2   # optional; falls back to AWS_REGION or ~/.aws/config
```

Activate:

```yaml
provider: bedrock-sonnet
```

## Authentication

Uses the AWS default credential chain — no `api_key` field needed:

1. Environment variables: `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_SESSION_TOKEN`
2. `~/.aws/credentials`
3. EC2/ECS IAM instance roles

```bash
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-west-2
```

## Model IDs

Bedrock model IDs follow the AWS naming convention and differ from direct Anthropic API names:

| Anthropic name | Bedrock model ID |
|----------------|-----------------|
| claude-sonnet-4-6 | `anthropic.claude-sonnet-4-6-v1:0` |

## Pricing

Built-in pricing table lookups may not match Bedrock IDs. Add explicit entries:

```yaml
pricing:
  "anthropic.claude-sonnet-4-6-v1:0":
    input_per_m: 3.0
    output_per_m: 15.0
```

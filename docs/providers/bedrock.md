# AWS Bedrock provider

See [README — Providers](https://github.com/Ricardo-M-L/anthrogo#providers).

The Bedrock provider calls Claude models via AWS Bedrock using standard AWS credential chains.

## Configuration

```yaml
provider: bedrock
region: us-east-1
model: anthropic.claude-opus-4-5-v1:0
```

Credentials are resolved via the standard AWS chain (env vars, `~/.aws/credentials`, EC2 instance role, etc.):

```bash
export AWS_ACCESS_KEY_ID=...
export AWS_SECRET_ACCESS_KEY=...
export AWS_REGION=us-east-1
```

(Full Bedrock provider reference migrating from README — M11.4 follow-up.)

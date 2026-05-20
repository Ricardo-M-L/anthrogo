# Google Vertex AI provider

See [README — Providers](https://github.com/Ricardo-M-L/anthrogo#providers).

The Vertex provider calls Claude models hosted on Google Cloud Vertex AI.

## Configuration

```yaml
provider: vertex
project: my-gcp-project
location: us-central1
model: claude-opus-4-5@20240229
```

Credentials are resolved via Application Default Credentials:

```bash
gcloud auth application-default login
# or
export GOOGLE_APPLICATION_CREDENTIALS=/path/to/service-account.json
```

(Full Vertex provider reference migrating from README — M11.4 follow-up.)

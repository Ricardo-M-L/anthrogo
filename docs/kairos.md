# KAIROS

See [README — KAIROS](https://github.com/Ricardo-M-L/anthrogo#kairos).

KAIROS (Knowledge-Augmented Intelligent Remote Orchestration System) enables dispatching agent tasks to remote machines within the same network.

## How it works

1. A KAIROS server runs on one or more remote machines.
2. The local anthrogo instance discovers available KAIROS nodes.
3. Tasks are dispatched to suitable nodes and results streamed back.

## Configuration

```yaml
kairos:
  nodes:
    - address: 10.0.0.10:9090
      name: gpu-node
```

## Dispatching tasks

```
/kairos "Run benchmarks on the GPU node"
```

(Full KAIROS reference migrating from README — M11.4 follow-up.)

## Signature verification (M11.9)

KAIROS supports ed25519 SSE signatures so clients can detect tampered streams.

### Generate a keypair

```bash
anthrogo --generate-key /path/to/kairos-key
# writes: /path/to/kairos-key.priv  (chmod 600)
#         /path/to/kairos-key.pub
```

### Start a signing worker

```bash
anthrogo --kairos-serve :9001 --signing-key /path/to/kairos-key.priv
```

### Connect with key pinning (global)

```bash
anthrogo --trust-key /path/to/kairos-key.pub
# or inline base64:
anthrogo --trust-key "$(cat /path/to/kairos-key.pub)"
```

### Per-subagent trust key (YAML)

```yaml
# .anthrogo/subagents/my-worker.yaml
name: my-worker
remote:
  endpoint: http://10.0.0.10:9001
  trust_key: "base64pubkeyhere=="
```

Per-spec `trust_key` takes precedence over `--trust-key`.

## TLS (M11.10)

KAIROS workers can serve HTTPS using a user-supplied certificate or via Let's Encrypt auto-provisioning.

### Plain HTTP (default)

```bash
anthrogo --kairos-serve :9001
# anthrogo kairos worker (HTTP — no TLS) on :9001
```

### TLS with a managed certificate

```bash
anthrogo --kairos-serve :443 --tls-cert /path/to/cert.pem --tls-key /path/to/key.pem
# anthrogo kairos worker (TLS) on :443
```

### Let's Encrypt (autocert)

Requires the server to be reachable on port 443 from the internet for the HTTP-01 challenge.

```bash
anthrogo --kairos-serve :443 --tls-auto --tls-domain worker.example.com
# anthrogo kairos worker (autocert TLS) on :443 for [worker.example.com]
```

Certificates are cached under `~/.anthrogo/autocert/`.

Multiple domains: `--tls-domain worker.example.com,worker2.example.com`.

### Clients — public CAs

Client connections to `https://` endpoints already work via stdlib CA roots with no extra configuration:

```yaml
# .anthrogo/subagents/my-worker.yaml
name: my-worker
remote:
  endpoint: https://worker.example.com/
```

### Clients — internal / self-signed CA

```yaml
name: my-worker
remote:
  endpoint: https://10.0.0.10:9001
  ca_cert_path: /path/to/internal-ca.pem
```

For development only (skips all certificate verification):

```yaml
name: my-worker
remote:
  endpoint: https://10.0.0.10:9001
  insecure_skip_verify: true  # DEV ONLY
```

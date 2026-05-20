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

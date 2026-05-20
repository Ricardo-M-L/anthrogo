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

# Installation

## Requirements

- Go 1.22+
- An API key for your chosen provider (Anthropic, DeepSeek, etc.) or local Ollama

## Build from source

```bash
git clone https://github.com/Ricardo-M-L/anthrogo.git
cd anthrogo
make build
./bin/anthrogo --version
```

## go install

```bash
go install github.com/Ricardo-M-L/anthrogo/cmd/anthrogo@latest
```

This places `anthrogo` in `$GOPATH/bin` (typically `~/go/bin`). Make sure that directory is on your `PATH`.

## Cross-platform release binaries

`make release` produces version-stamped binaries for all four platforms under `dist/`:

```
dist/anthrogo-<version>-darwin-amd64
dist/anthrogo-<version>-darwin-arm64
dist/anthrogo-<version>-linux-amd64
dist/anthrogo-<version>-linux-arm64
```

Pre-built binaries for tagged releases are attached to [GitHub releases](https://github.com/Ricardo-M-L/anthrogo/releases). Download and add to your PATH.

## Homebrew (planned)

A Homebrew tap is planned for a future milestone. For now, use `go install` or build from source.

## Next step

After installation, run [first run](first-run.md) to configure anthrogo and verify your environment.

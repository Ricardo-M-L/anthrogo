# Installation

## Requirements

- Go 1.26+ (chromedp/cdproto pins this minimum) — only for building from
  source. Homebrew / direct-download binaries have no Go runtime
  requirement.
- An API key for your chosen provider (Anthropic, DeepSeek, etc.) or local
  Ollama.

## Homebrew (recommended)

```bash
brew install Ricardo-M-L/tap/anthrogo
```

Works on macOS (Intel + Apple Silicon) and Linux. Brew handles the macOS
quarantine attribute automatically.

## Prebuilt binaries

Grab the asset matching your platform from
[releases/latest](https://github.com/Ricardo-M-L/anthrogo/releases/latest):

| Platform | Asset |
|---|---|
| macOS Apple Silicon | `anthrogo_<ver>_darwin_arm64.tar.gz` |
| macOS Intel | `anthrogo_<ver>_darwin_x86_64.tar.gz` |
| Linux x86_64 | `anthrogo_<ver>_linux_x86_64.tar.gz` |
| Linux ARM64 | `anthrogo_<ver>_linux_arm64.tar.gz` |

```bash
tar -xzf anthrogo_<ver>_<os>_<arch>.tar.gz
sudo mv anthrogo /usr/local/bin/
anthrogo version
```

### macOS Gatekeeper note (direct download only)

anthrogo binaries are **not yet Apple-notarized** (notarization requires
a paid Apple Developer account). On first run from a direct download,
Gatekeeper may flag the binary as unverified.

```bash
# Strip the quarantine attribute:
xattr -dr com.apple.quarantine /usr/local/bin/anthrogo
```

Homebrew installs bypass this issue automatically.

## Docker

```bash
docker run --rm -p 8765:8765 ghcr.io/ricardo-m-l/anthrogo:latest
curl http://localhost:8765/v1/health
```

The container defaults to `anthrogo serve --addr 0.0.0.0:8765`. Override
with any other entrypoint (e.g. headless mode):

```bash
docker run --rm \
  -e ANTHROPIC_API_KEY=$ANTHROPIC_API_KEY \
  ghcr.io/ricardo-m-l/anthrogo:latest \
  -p "summarize this README"
```

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

## Next step

After installation, run [first run](first-run.md) to configure anthrogo and verify your environment.

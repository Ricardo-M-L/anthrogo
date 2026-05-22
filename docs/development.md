# Development guide

## Prerequisites

- Go 1.26+ (chromedp/cdproto pins this minimum)
- `make`
- Optional: `golangci-lint` (`brew install golangci-lint`)
- Optional: `mkdocs` + `mkdocs-material` (`pip install mkdocs mkdocs-material`)

## Make targets

Run `make help` to list all targets with descriptions. Key targets:

| Target | Description |
|---|---|
| `make build` | Compile `bin/anthrogo` for the current platform |
| `make test` | Run all unit + integration tests |
| `make race` | Run tests with `-race` on hot packages |
| `make sweep` | Three uncached test sweeps — catches flaky tests |
| `make vet` | Run `go vet` over all packages |
| `make lint` | Run `golangci-lint` (requires install) |
| `make fmt` | Format with `gofmt` + `goimports` |
| `make install` | Install to `$GOPATH/bin` |
| `make release` | Cross-compile for darwin/linux × amd64/arm64 |
| `make api-docs` | Generate `docs/api/` from `go doc -all` |
| `make bench` | Run benchmark suite with `-benchmem` |
| `make clean` | Remove `bin/` and `dist/` |

## API docs generation

```bash
make api-docs
# or directly:
./scripts/gen-api-docs.sh
```

Writes one Markdown file per package under `docs/api/`. The index page is `docs/api/index.md`. To preview:

```bash
mkdocs serve
```

## Benchmarks

```bash
make bench
# or with more iterations:
go test -bench=. -benchmem -benchtime=3s ./pkg/tokens ./pkg/compact ./pkg/bashscan ./internal/session ./pkg/permissions
```

Benchmarks are defined in `bench_test.go` files in each package. Use
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) to compare runs:

```bash
go test -bench=. -count=5 ./pkg/tokens | tee old.txt
# make changes...
go test -bench=. -count=5 ./pkg/tokens | tee new.txt
benchstat old.txt new.txt
```

## pprof profiling

Pass `--pprof localhost:6060` to enable `net/http/pprof` while the TUI is running:

```bash
./bin/anthrogo --pprof localhost:6060
# in another terminal:
go tool pprof http://localhost:6060/debug/pprof/heap
```

All standard pprof endpoints (`/debug/pprof/`, `/debug/pprof/goroutine`, etc.) are available. Bind to `localhost` only — do not expose this port publicly.

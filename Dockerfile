# syntax=docker/dockerfile:1.7

# Build stage — pinned Go toolchain matching go.mod's directive.
FROM golang:1.26-alpine AS build

WORKDIR /src

# Cache module download separately from source builds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .

ARG VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build \
      -ldflags "-s -w -X github.com/ricardo/anthrogo/internal/version.Version=${VERSION}" \
      -o /out/anthrogo \
      ./cmd/anthrogo

# Runtime stage — distroless minimises CVE surface.
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

LABEL org.opencontainers.image.source="https://github.com/Ricardo-M-L/anthrogo"
LABEL org.opencontainers.image.description="anthrogo — a Go port of Anthropic's Claude Code CLI"
LABEL org.opencontainers.image.licenses="AGPL-3.0-or-later"

COPY --from=build /out/anthrogo /usr/local/bin/anthrogo

USER nonroot:nonroot
WORKDIR /workspace

# Default to the HTTP daemon on container, since it's the most useful entry
# in a container context. TUI mode requires a terminal; web mode opens a
# browser. Override with `docker run ... anthrogo -p '...'` for headless.
EXPOSE 8765
ENTRYPOINT ["/usr/local/bin/anthrogo"]
CMD ["serve", "--addr", "0.0.0.0:8765"]

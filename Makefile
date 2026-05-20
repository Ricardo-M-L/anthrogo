BIN := bin/anthrogo
PKG := ./cmd/anthrogo
VERSION := $(shell grep 'var Version' internal/version/version.go | sed -E 's/.*"(.*)".*/\1/')
LDFLAGS := -X github.com/ricardo/anthrogo/internal/version.Version=$(VERSION)

# Release matrix
PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64
RELEASE_DIR := dist

.PHONY: build test vet fmt clean install lint race sweep release help

help: ## Show this help.
	@grep -E '^[a-zA-Z_-]+:.*## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build anthrogo for the current platform.
	@mkdir -p bin
	go build -ldflags '$(LDFLAGS)' -o $(BIN) $(PKG)

test: ## Run all unit + integration tests.
	go test ./...

race: ## Run race detector on hot packages.
	go test -race -count=2 ./pkg/query ./pkg/tool ./internal/tui ./internal/session ./pkg/command ./internal/mcp ./internal/system ./internal/hooks ./pkg/permissions

sweep: ## 3x uncached test sweep (catches flakes).
	@for i in 1 2 3; do \
		echo "=== run $$i ==="; \
		go clean -testcache; \
		go test ./... 2>&1 | grep -E "FAIL|^FAIL" || echo "clean"; \
	done

vet: ## go vet all packages.
	go vet ./...

lint: ## Run golangci-lint (install first via 'brew install golangci-lint' if missing).
	@if ! command -v golangci-lint > /dev/null; then \
		echo "golangci-lint not on PATH. Install: brew install golangci-lint"; \
		exit 1; \
	fi
	golangci-lint run --timeout=5m

fmt: ## Format Go code.
	gofmt -w .
	@which goimports > /dev/null && goimports -w . || true

clean: ## Remove build artifacts.
	rm -rf bin $(RELEASE_DIR)

install: ## go install to $GOPATH/bin.
	go install -ldflags '$(LDFLAGS)' $(PKG)

release: ## Cross-compile release binaries for darwin/linux × amd64/arm64.
	@mkdir -p $(RELEASE_DIR)
	@for p in $(PLATFORMS); do \
		GOOS=$$(echo $$p | cut -d/ -f1); \
		GOARCH=$$(echo $$p | cut -d/ -f2); \
		OUT=$(RELEASE_DIR)/anthrogo-$(VERSION)-$$GOOS-$$GOARCH; \
		echo "→ $$OUT"; \
		GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags '$(LDFLAGS)' -o $$OUT $(PKG); \
	done
	@ls -la $(RELEASE_DIR)/

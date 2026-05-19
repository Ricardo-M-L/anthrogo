BIN := bin/anthrogo
PKG := ./cmd/anthrogo

.PHONY: build test vet fmt clean install

build:
	@mkdir -p bin
	go build -o $(BIN) $(PKG)

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .
	goimports -w . 2>/dev/null || true

clean:
	rm -rf bin

install:
	go install $(PKG)

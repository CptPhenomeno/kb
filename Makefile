# kb developer tasks. `make check` is the gate every task must pass.

BIN        := bin/kb
PKG        := github.com/CptPhenomeno/kb
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION)

.PHONY: all fmt fmt-check vet lint test check build clean

all: check build

## fmt: rewrite Go sources with gofmt
fmt:
	gofmt -w .

## fmt-check: fail if any Go source is not gofmt-formatted
fmt-check:
	@out="$$(gofmt -l .)"; if [ -n "$$out" ]; then echo "not gofmt-formatted:"; echo "$$out"; exit 1; fi

## vet: run go vet
vet:
	go vet ./...

## lint: run staticcheck (declared as a tool in go.mod)
lint:
	go tool staticcheck ./...

## test: run all tests with the race detector
test:
	go test -race ./...

## check: formatting, vet, lint and tests
check: fmt-check vet lint test

## build: build a static kb binary into bin/
build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o $(BIN) ./cmd/kb

## clean: remove the built binary
clean:
	rm -f $(BIN)

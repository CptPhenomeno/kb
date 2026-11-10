# kb developer tasks. `make check` is the gate every task must pass.

BIN        := bin/kb
PKG        := github.com/CptPhenomeno/kb
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X $(PKG)/internal/buildinfo.Version=$(VERSION)
DIST       := dist
PLATFORMS  := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64

.PHONY: all fmt fmt-check vet lint test check build cross clean

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

## cross: build static kb binaries for every os/arch in PLATFORMS into
## $(DIST)/kb-<os>-<arch>
cross:
	@set -e; for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; out=$(DIST)/kb-$(VERSION)-$$os-$$arch; \
		echo "building $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags '$(LDFLAGS)' -o $$out ./cmd/kb; \
	done

## clean: remove the built binary and the cross builds
clean:
	rm -f $(BIN) $(DIST)/kb-*

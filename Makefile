.PHONY: build test lint ci clean

BINARY := rufus
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags="-s -w -X github.com/cjbarker/rufus/cmd.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) .

test:
	go test -v -race ./...

lint:
	golangci-lint run ./...

ci: lint test build

clean:
	rm -f $(BINARY)
	go clean -cache -testcache

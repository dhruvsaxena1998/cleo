.PHONY: build build-release test lint run clean

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
VERSION_FLAG := -X github.com/dhruvsaxena1998/cleo/internal/cli.Version=$(VERSION)

build:
	go build -ldflags="$(VERSION_FLAG)" -o bin/cleo ./cmd/cleo

build-release:
	go build -ldflags="-s -w $(VERSION_FLAG)" -o bin/cleo ./cmd/cleo

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/cleo

clean:
	rm -rf bin/

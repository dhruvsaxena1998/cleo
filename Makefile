.PHONY: build build-release test lint run clean

build:
	go build -o bin/cleo ./cmd/cleo

build-release:
	go build -ldflags="-s -w" -o bin/cleo ./cmd/cleo

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/cleo

clean:
	rm -rf bin/

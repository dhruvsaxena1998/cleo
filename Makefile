.PHONY: build test lint run clean

build:
	go build -o bin/cleo ./cmd/cleo

test:
	go test ./...

lint:
	go vet ./...

run: build
	./bin/cleo

clean:
	rm -rf bin/

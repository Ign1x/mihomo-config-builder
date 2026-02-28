BINARY=bin/mcb

.PHONY: build test lint ci fmt

build:
	go build -o $(BINARY) ./cmd/mcb

test:
	go test ./...

lint:
	go vet ./...

fmt:
	gofmt -w ./cmd ./internal

ci: lint test

build:
    go build -o bin/mcb ./cmd/mcb

test:
    go test ./...

lint:
    go vet ./...

fmt:
    gofmt -w ./cmd ./internal

ci: lint test

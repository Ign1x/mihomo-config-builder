build:
    GOPROXY=off GOSUMDB=off CGO_ENABLED=0 go build -mod=vendor -o bin/mcb ./cmd/mcb

test:
    GOPROXY=off GOSUMDB=off go test -mod=vendor ./...

lint:
    GOPROXY=off GOSUMDB=off go vet -mod=vendor ./...

fmt:
    gofmt -w ./cmd ./internal

ci: lint test

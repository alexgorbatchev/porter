default:
    @just --list

run:
    go run ./cmd/porter

help:
    go run ./cmd/porter --help

version:
    go run ./cmd/porter --version

test:
    go test ./...

vet:
    go vet ./...

lint:
    golangci-lint run

build:
    mkdir -p bin
    go build -o bin/porter ./cmd/porter

verify:
    go mod tidy -diff
    go build ./...
    go vet ./...
    go test -race ./...
    golangci-lint run

# Download project dependencies for local development
setup:
    go mod download

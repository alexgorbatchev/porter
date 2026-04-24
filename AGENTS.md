# porter

Linux Go CLI that lists listening TCP ports for processes whose current working directory matches the current repo directory or one of its descendants.

## Commands
- Just list: `just`
- Run: `just run`
- Help: `just help`
- Version: `just version`
- Verify: `just verify`
- Test: `go test ./...`
- Vet: `go vet ./...`
- Lint: `golangci-lint run`
- Build: `mkdir -p bin && go build -o bin/porter ./cmd/porter`
- Run directly: `go run ./cmd/porter`

## Setup
- Use Go 1.26+.
- The CLI is Linux-only at runtime because listener discovery uses Linux socket diagnostics and `/proc` process metadata.
- Local runs print `porter dev`; tagged release binaries get their version from GoReleaser via `-X main.version={{.Version}}`.

## Conventions
- Keep OS wiring in `cmd/porter/main.go`; keep flags, terminal loop, and rendering in `internal/cli/`.
- Keep socket-diag listener discovery, `/proc` process inspection, and command formatting in `internal/procfs/`.
- Show commands relative to the current cwd when absolute paths fall under that tree.

## Gotchas
- `/proc/<pid>/cwd` is the process's current working directory, not a durable launch-origin record; do not claim stronger semantics than Linux provides.
- `x` sends `SIGTERM`, not `SIGKILL`; if stronger kill semantics are needed, treat that as a product change and ask first.

## Boundaries
- Always: run `go test ./...`, `go vet ./...`, and `go build ./...` after changing Go code or GitHub workflows.
- Ask first: module path changes, dependency additions, signal-semantics changes, or any attempt to support non-Linux platforms.
- Never: commit `.tmp/`, commit `dist/`, or shell out to `lsof`/`ss` for core listener discovery without approval.

## References
- `README.md`
- `internal/cli/run.go`
- `internal/procfs/listeners.go`
- `.github/workflows/ci.yml`
- `.github/workflows/release.yml`
- `.goreleaser.yml`

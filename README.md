# `porter`

`porter` is a Linux CLI that shows listening TCP ports owned by processes whose current working directory matches the directory where `porter` is started, or one of its descendants.

It is aimed at local development workflows where commands such as `bun dev`, `vite`, `go run`, or `python -m http.server` are launched from a repo and you want to see which PIDs are still serving ports.

## Quick start

Run `porter` from the project directory you want to inspect:

```sh
porter
```

The dashboard lists:

- `PID`
- `PORT`
- the started command, with absolute paths under the current directory rewritten relative to that directory

If the listening process is running from a subdirectory, `porter` prefixes the command with that relative directory:

```text
1234  3000  apps/web $ node_modules/.bin/bun dev
```

## Controls

- `j` / `k`: move the selection down/up
- `x`: send `SIGTERM` to the selected PID
- `r`: refresh immediately
- `q`: quit

`porter` polls periodically and performs a quick follow-up refresh after sending `SIGTERM`.

## Matching rules

`porter` uses Linux socket diagnostics plus `/proc` process metadata for discovery:

- It queries `NETLINK_SOCK_DIAG` (`sock_diag(7)`) for listening TCP sockets.
- It matches socket inodes against `/proc/<pid>/fd/*` symlinks.
- It filters processes by `/proc/<pid>/cwd`.

Important limitation: `/proc/<pid>/cwd` is the process's **current** working directory, not a historical “started from here” record. If a process changes directory after launch, `porter` can only report the current location that Linux exposes.

`porter` skips processes whose `/proc` metadata is unreadable instead of failing the whole dashboard refresh.

## Development

Go 1.26 or newer is required for local development.

Common commands are available through `just`:

- Run CLI: `just run`
- Help: `just help`
- Version: `just version`
- Test: `just test`
- Vet: `just vet`
- Lint: `just lint`
- Build: `just build`
- Full verification: `just verify`

`just build` writes the CLI binary to `bin/porter`.

Equivalent Go commands:

- Test: `go test ./...`
- Vet: `go vet ./...`
- Lint: `golangci-lint run`
- Build: `mkdir -p bin && go build -o bin/porter ./cmd/porter`

## Releases

GitHub Actions workflows are included for CI and tagged releases. Release builds inject the runtime version into `main.version`, so `porter --version` reports the tag value in published binaries.

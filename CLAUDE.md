# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`pretty` is a parallel remote execution TTY (parallel SSH shell) written in Go. It opens persistent SSH shell sessions to multiple hosts, fans commands out to all of them, and displays color-prefixed output in an interactive terminal UI built on Bubbletea v2.

See `AGENTS.md` for build/test/lint commands and code style guidelines.

## Common Development Commands

```bash
go build -v ./...                # build all packages
go build -o pretty .             # build binary
go test -v ./...                 # full test suite
go test -v ./internal/shell      # single package
go test -v ./internal/shell -run '^TestParseCommandAsync$'  # single test
go test -race ./...              # race detection (run when touching concurrent code)
go test -coverpkg=./... ./... -race -coverprofile=coverage.out -covermode=atomic  # CI coverage
gofmt -w . && go vet ./...       # format + lint
```

Go 1.26 required (see `go.mod`). CI runs on Ubuntu with `go-version: '1.26'`.

## Architecture

### Data Flow

```
CLI (cmd/root.go)
  -> parse host specs, resolve SSH config, build HostList
  -> shell.Spawn(hostList)
       -> start Broker goroutine (fan-out)
       -> start Bubbletea TUI program

User input -> model.Update -> CommandRequest -> broker channel
  -> per-host worker goroutine -> SSH stdin

SSH stdout -> ProxyWriter -> OutputEvent channel
  -> model.Update (outputMsg) -> viewport
```

### Key Packages

- **`cmd/`** -- Cobra CLI. Host spec parsing (`hosts.go`), SSH config resolution, group/file loading, color assignment. Entry point calls `shell.Spawn`.
- **`internal/shell/`** -- Bubbletea v2 TUI. The `model` struct owns the text input, viewport, output buffer, command history, and job manager. Commands are parsed in `command.go` (`:async`, `:status`, `:list`, `:scroll`, `:bye`, `exit`, or plain text = run).
- **`internal/sshConn/`** -- SSH connection lifecycle. `Broker` fans a single `CommandRequest` channel out to per-host `worker` goroutines. Each worker holds a persistent SSH shell session. `RunCommand` opens a fresh session for async jobs. `config.go` resolves SSH config (Host/Match patterns, ProxyJump, IdentityFile) via `ncode/ssh_config`.
- **`internal/jobs/`** -- Job tracking. `Manager` maintains the latest normal job and last 2 async jobs. Thread-safe via mutex + snapshot cloning for reads. `sentinel.go` injects/extracts `__PRETTY_EXIT__<jobID>:<exitCode>` markers to capture per-host exit codes from shell output.

### Concurrency Model

- **Broker pattern**: One goroutine reads from the shared `CommandRequest` channel and forwards to each connected host's private channel.
- **Per-host workers**: Each host has a dedicated goroutine holding an SSH shell session, reading from its private channel.
- **Async jobs**: Each `:async` command spawns N goroutines (one per connected host), each opening a fresh SSH session via `RunCommand`.
- **Connection state**: `Host.IsConnected` and `Host.IsWaiting` are `int32` atomics.
- **Job snapshots**: `Manager` uses copy-on-read (`cloneJob`) so the Bubbletea model never holds a lock while rendering.

### Sentinel Protocol

Commands sent to remote shells are wrapped with a trailing `printf '__PRETTY_EXIT__<jobID>:%d\n' $?`. The `ExtractSentinel` function in `internal/jobs/sentinel.go` parses these markers from output lines to determine per-host exit codes and mark jobs complete.

### TUI (Bubbletea v2)

Uses `charm.land/bubbletea/v2` and `charm.land/bubbles/v2` (not the old `github.com/charmbracelet` import paths). The view is an alt-screen with a viewport (output) + text input (prompt). Scroll mode (`:scroll`) disables auto-follow and lets the user scroll the viewport; `esc` returns to the prompt.

## Testing

- `internal/sshConn` uses function variables (`connectionFunc`, `sessionFunc`, `workerRunner`) to stub SSH calls in tests.
- `cmd/root.go` uses `loadSSHConfigFunc`, `resolveHostFunc`, and `spawnShellFunc` for the same purpose.
- Table-driven tests throughout; use `t.Run` with descriptive subtest names.

## Local SSHD Testbed

For integration testing against real SSH servers:
```bash
make demo    # setup + docker + build + run (full workflow)
make clean   # tear down containers and remove .pretty-test/
```

Or step by step:
```bash
export PRETTY_AUTHORIZED_KEY="$(ssh-add -L | grep 'my-key' | head -n1)"  # optional
make testbed      # setup + docker + host key scan + build
make run          # launch pretty against testbed
make testbed-down # stop containers
```

# AGENTS.md

Guidance for agentic coding assistants working in `github.com/ncode/pretty`.

## Quick Start for Agents

Run these first from repo root:

1. `go env -w GOTOOLCHAIN=go1.25.0+auto`
2. `gofmt -w .`
3. `go test -v ./...`

When narrowing scope:

- Single package: `go test -v ./internal/shell`
- Single test: `go test -v ./internal/shell -run '^TestParseCommandAsync$'`
- Subtest: `go test -v ./cmd -run '^TestParseHostSpec$/host_with_port$'`

## Project Snapshot

- Language: Go (`go 1.25` in `go.mod`)
- Binary entrypoint: `main.go`
- CLI layer: `cmd/`
- Core packages: `internal/shell`, `internal/sshConn`, `internal/jobs`
- Main user docs: `README.md`

## Toolchain and Environment

- Use Go 1.25.
- Match CI behavior when possible:
  - `go env -w GOTOOLCHAIN=go1.25.0+auto`
- A `Makefile` is provided. Run `make` to build, `make test` for tests, `make demo` for the full testbed workflow.
- No dedicated linter config (`.golangci.yml`) is present.

## Build / Lint / Test Commands

Run from repository root.

### Build

- Build all packages: `go build -v ./...`
- Build binary only: `go build -o pretty .`

### Test (standard)

- Full suite: `go test -v ./...`
- Full suite with race detector: `go test -race ./...`
- CI-like coverage command:
  - `go test -coverpkg=./... ./... -race -coverprofile=coverage.out -covermode=atomic`

### Test (single package / file / test)

- Single package:
  - `go test -v ./internal/shell`
- Single test function:
  - `go test -v ./internal/shell -run '^TestParseCommandAsync$'`
- Single test in another package:
  - `go test -v ./cmd -run '^TestParseHostSpec$'`
- Subtest target (table-driven tests):
  - `go test -v ./cmd -run '^TestParseHostSpec$/host_with_port$'`
- Repeat a flaky test deterministically:
  - `go test -v ./internal/sshConn -run '^TestResolveHostPatternMatchWildcard$' -count=1`

### Lint / Static checks

Because no project-specific linter config exists, use baseline Go checks:

- Format check and rewrite: `gofmt -w .`
- Vet all packages: `go vet ./...`
- Optional stronger check (if installed): `staticcheck ./...`

If a change touches many files, run at least:

1. `gofmt -w .`
2. `go test -v ./...`

If concurrency/networking code changes, also run:

3. `go test -race ./...`

## Workflow Expectations for Agents

- Prefer small, focused diffs.
- Do not introduce new dependencies unless clearly necessary.
- Preserve existing CLI behavior and command semantics.
- Keep public behavior aligned with `README.md`.
- Add or update tests when behavior changes.

## Code Style Guidelines

These reflect patterns already used in this repository.

### Formatting and file layout

- Always use `gofmt` formatting.
- Keep package names short and lowercase; follow existing names (including `sshConn`).
- Keep functions cohesive and avoid deep nesting.
- Prefer early returns to reduce indentation.

### Imports

- Group imports in three blocks when needed:
  1. Go standard library
  2. Third-party libraries
  3. Internal project imports (`github.com/ncode/pretty/...`)
- Keep imports sorted as `gofmt` outputs.
- Use import aliases only when needed for clarity or conflicts (for example `tea`, `homedir`).

### Naming

- Exported identifiers: PascalCase.
- Unexported identifiers: camelCase.
- Constants:
  - exported: PascalCase if part of API
  - internal/private: camelCase (`defaultPrompt`, `maxOutputLines`)
- Test names: `TestXxx` with descriptive suffixes.

### Types and data structures

- Prefer concrete structs for domain state (`Manager`, `HostSpec`, `ResolvedHost`).
- Use pointers for shared/mutable state.
- Initialize slices/maps with capacity when size is known.
- Keep zero-value behavior sensible.

### Error handling

- In library/internal packages:
  - return errors to caller
  - wrap with context using `%w` when rethrowing (`fmt.Errorf("...: %w", err)`).
- In CLI command execution paths (`cmd/`), current pattern is:
  - print user-facing error
  - exit non-zero (`os.Exit(1)`).
- Avoid panics for expected runtime errors.

### Concurrency and synchronization

- Guard shared mutable state with `sync.Mutex`/atomics as currently done.
- Keep lock scope tight; avoid blocking operations while holding locks.
- For goroutines in tests/helpers, use `t.Cleanup` to shut down resources.

### Testing conventions

- Prefer table-driven tests for parser/validation behavior.
- Use `t.Run` with stable, descriptive case names.
- Use `t.Helper()` in test helpers.
- Use `t.Fatalf` for fatal assertions; include expected vs got details.
- Prefer deterministic tests; avoid sleeps unless absolutely required.

### Comments and docs

- Add comments for non-obvious logic, invariants, or protocol details.
- Do not add redundant comments that restate code.
- Keep exported APIs understandable from names and function signatures.

## Validation Checklist Before Finishing

- Code is `gofmt`-formatted.
- Relevant package tests pass.
- Full `go test -v ./...` passes for non-trivial changes.
- `go test -race ./...` run when touching concurrent code.
- `README.md` updated if CLI behavior or config format changed.

## Repository-Specific Rules Files

Checked paths:

- `.cursor/rules/`
- `.cursorrules`
- `.github/copilot-instructions.md`

Current status in this repo: none of the above files exist.

If any are added later, treat their guidance as authoritative and merge it into this document.

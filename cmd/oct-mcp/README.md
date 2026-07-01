# oct-mcp

A Model Context Protocol (MCP) server exposing the Oct toolchain, built with
the official `github.com/modelcontextprotocol/go-sdk`.

## Design: in-process, not subprocess

`cmd/oct-mcp` lives *inside* the Oct module (`github.com/yuechen-li-dev/oct`)
specifically so it can `import "github.com/yuechen-li-dev/oct/internal/cli"`.
Go's `internal/` import-visibility rule restricts that package to importers
within this module tree — a separately-`go mod init`'d MCP server could not
reach it, and would be limited to shelling out to a pre-built `oct` binary
via `os/exec` (subprocess spawn, PATH lookup, text-scraping stdout).

Instead, every tool below calls the exact function `cmd/oct/main.go` calls:

```go
func Execute(args []string, stdout io.Writer, stderr io.Writer) error
```

in-process, against `bytes.Buffer`s instead of the real process streams. This
keeps the MCP tool surface automatically in sync with the CLI's own flag
parsing (`parseTestOptions`, `parseArtifactOptions`, etc.) rather than
duplicating `tester`/`build`/`run` logic at a lower level, while avoiding
subprocess overhead entirely.

## Tools

| Tool | Wraps | Notes |
|---|---|---|
| `oct_test` | `oct test [--suite] [--execution] [--all-packages] <path>` | Runs `[Fact]`s |
| `oct_build` | `oct build <path>` | Compile-only, reports typecheck/compile errors |
| `oct_run` | `oct run <path>` | Executes a program, captures stdout/stderr |
| `oct_artifact` | `oct artifact [--execution] <path>` | Runs `[Artifact]` blocks, produces `.octagon` files |
| `oct_new` | `oct new <kind> <name> <dir>` | Scaffolds a new experiment/library/app |
| `oct_fmt_check` | `oct fmt --check <path>` | Read-only formatting check |

Every tool call returns `execErr != nil` (surfaced as `IsError: true`) for
both usage errors *and* command-reported failures (failing tests, compile
errors) — the model sees the real Oct output either way, it's just flagged
as an error result when the command itself failed.

### `oct_new` and working-directory safety

`oct new`'s normal behavior auto-places new packages under `Experiments/`,
`Libraries/`, or `Applications/` relative to the process's current working
directory when no explicit `[path]` is given. That's process-global mutable
state, which is unsafe to rely on across concurrent MCP tool calls from a
long-lived server process. `oct_new` requires an explicit `dir` argument for
every call instead, so the tool has no CWD dependency at all.

### Deliberately out of scope (v1)

- **`oct_fmt_write`** (mutating format-in-place) — kept separate from
  `oct_fmt_check` so read-only tools ship first; a mutating variant is a
  natural, small follow-up.
- **`oct pkg` / `oct exp` (dependency sync, running remote experiment
  repos)** — these can fetch from git/network sources; deferred pending a
  decision on what network access an MCP tool call should be allowed to
  trigger implicitly.
- **`oct bench`** — cheap to add (same shape as `oct_test`), just not
  included in this first pass.

## Build

```sh
go build -o oct-mcp ./cmd/oct-mcp
```

Requires Go >= 1.25.0 (pulled in transitively by the MCP SDK's `go.mod`;
`go build`/`go run` will fetch the toolchain automatically if `GOTOOLCHAIN`
allows it).

## Run

`oct-mcp` speaks MCP over stdio. Point any MCP client at the built binary,
e.g. for local testing with the SDK's own client:

```go
transport := &mcp.CommandTransport{Command: exec.Command("./oct-mcp")}
session, _ := client.Connect(ctx, transport, nil)
res, _ := session.CallTool(ctx, &mcp.CallToolParams{
    Name:      "oct_test",
    Arguments: map[string]any{"path": "Libraries/Uncertainty"},
})
```

## Verified

- `go build ./cmd/oct-mcp` — clean.
- `go build ./cmd/oct` and `go vet ./...` — clean (no regressions from the
  added `go-sdk` dependency).
- A standalone MCP client (SDK, `CommandTransport`, stdio) connected to the
  built server: `ListTools` correctly enumerated all 6 tools; `oct_test` on
  `Libraries/Uncertainty` returned the identical 18/18-pass result as
  running `oct test Libraries/Uncertainty` directly; `oct_build` on a
  missing file correctly returned `IsError: true` with the real compiler
  message (`source file not found: ...`).

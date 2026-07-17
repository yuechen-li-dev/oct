# Oct MCP

`oct-mcp` gives hosted ChatGPT clients bounded access to the real Oct CLI in a
fresh virtual workspace. It is an MCP server, not a shell or filesystem bridge.

## Local Codex

For local Codex, use the repository-native workflow first:

```powershell
oct test <target> --execution auto --json
oct artifact <target> --execution interpreted --json
```

The plugin provides `oct-workflow` for libraries and ordinary repository work,
and `oct-experiments` for `oct new experiment` plus M0/M1 evidence work. Build
`oct-mcp` only when a bounded virtual-workspace helper is useful:

```powershell
go build -o oct-mcp ./cmd/oct-mcp
```

The plugin config invokes `oct-mcp --stdio`; on Windows use `oct-mcp.exe`.
This is not a replacement for repository editing, local artifact inspection,
or the normal CLI.

Protocol traffic uses stdout only. Operational logs use stderr. Requests accept
only bounded virtual `.oct`, `.octest`, and `.octfail` files, never host paths.

## Hosted use

Run `oct-mcp serve --listen :8080`. It serves the current MCP streamable-HTTP transport at `/mcp` and liveness at `/healthz`. Put it behind TLS termination, an authenticated/rate-limited ingress, and the container/OS sandbox described in [SECURITY.md](SECURITY.md). Do not expose a developer workstation.

## Public tools

| Tool | Purpose |
| --- | --- |
| `oct_workspace_info` | Virtual-workspace limits, packaged runtime imports, and the canonical hosted workflow. |
| `oct_test` | Canonical validation through `oct test`; reports structured diagnostics and compiled/interpreted fallback. |
| `oct_artifact` | Canonical evidence generation through `oct artifact`; returns scoped artifact metadata. |
| `oct_run` | Bounded hosted playground execution of a `.oct` entry only. |
| `oct_get_artifact` | Retrieve one authorized, unexpired artifact by execution and artifact ID. |

All tools return the same structured envelope: protocol/compiler/command identity,
`ok`, execution ID, diagnostics, bounded stdout/stderr, timings, limits,
provenance, and artifacts where meaningful. `oct_test` and `oct_artifact`
include the real CLI's `oct.cli.result.v1` result rather than inventing a
second semantic authority. The server exposes no prompts or filesystem resource.

Use `oct_test` for normal hosted repair loops. An artifact ID is capability-like
only within its server process and retention period; it is never a host path.

## Compatibility and versioning

The server and plugin are `0.1.0`; tool schema is `2.0`. This schema major
replaces the speculative compiler-operation surface with canonical workflow
tools. Oct itself remains a 0.1 preview; `oct_workspace_info` reports runtime
limits and modes.

See [DEPLOYMENT.md](DEPLOYMENT.md), [SECURITY.md](SECURITY.md), and [PUBLICATION_CHECKLIST.md](PUBLICATION_CHECKLIST.md).

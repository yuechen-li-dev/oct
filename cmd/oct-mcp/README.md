# oct-mcp

`oct-mcp` is the hosted Oct Model Context Protocol server. It executes each public request in a fresh temporary virtual workspace through the real `internal/cli` command path. Local Codex should normally edit its repository and invoke `oct test` / `oct artifact` directly.

## Commands

```text
oct-mcp --stdio
oct-mcp serve --listen :8080
```

Stdio is for local Codex/ChatGPT clients and writes MCP frames only to stdout. Streamable HTTP serves `/mcp`; `/healthz` is a liveness endpoint for an ingress. TLS, authentication/abuse controls, OS/container isolation and rate limiting belong at deployment boundaries.

## Tool contract

`oct_workspace_info`, `oct_test`, `oct_artifact`, `oct_run`, and `oct_get_artifact` are the stable hosted surface. `oct_test` and `oct_artifact` wrap the canonical CLI lanes and preserve their structured `oct.cli.result.v1` output. `oct_run` is deliberately playground-only. Source tools take source text or a bounded list of virtual project files, never a host path. Results use a shared structured envelope with protocol/server/compiler identity, explicit compiled/interpreted behavior, diagnostics, execution identity, timing, limits, provenance, and authorized artifacts.

See [the MCP documentation](../../docs/mcp/README.md) for local setup, hosted deployment, security limits and publication materials.

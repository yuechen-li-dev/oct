# Oct MCP deployment

Build the container with `docker build -f Dockerfile.oct-mcp -t oct-mcp:0.1.0 .`, then run it behind a managed HTTPS ingress that routes `/mcp` and `/healthz`.

Required production settings: non-root container, read-only root filesystem where the Oct runtime permits it, writable `emptyDir`/tmpfs only for `/tmp`, disabled egress, CPU/memory/PID limits, request-body limit of 1 MiB, TLS at ingress, rate limits, and structured/redacted logs. Run one region for MVP and use rolling deployment/immutable image tags for rollback.

The server intentionally has no built-in end-user authentication. A public deployment must apply anonymous abuse quotas at ingress; if identity/quota ownership is required, use standards-compliant OAuth/OIDC at the ingress or MCP resource boundary rather than a custom token protocol. HTTPS terminates at ingress; `--listen` is plain HTTP for the internal pod/container network.

The image carries the reviewed `Libraries/` runtime source used by hosted
virtual workspaces. Readiness is `GET /healthz` returning 200. A successful
health response only proves the MCP process is alive; release validation must
invoke `/mcp`, run `oct_test`, generate an `oct_artifact`, and retrieve it.

### Local smoke test

`docker run --rm -p 8080:8080 --read-only --tmpfs /tmp:rw,noexec,nosuid,size=64m oct-mcp:0.1.0 serve --listen :8080`

The exact ingress, domain, TLS certificate, rate limiter, privacy URL and support contact are deployment-owner inputs and are listed in the publication manifest.

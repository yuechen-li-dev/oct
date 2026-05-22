# Release Checklist (Go module / pkg.go.dev)

Use this checklist for public module releases.

1. Ensure CI is green:

   ```bash
   go test ./... -count=1
   ```

2. Verify local installs:

   ```bash
   go install ./cmd/oct
   go install ./cmd/octxiliary-io
   ```

3. Tag and publish:

   ```bash
   git tag v0.1.0
   git push origin v0.1.0
   ```

4. Trigger pkg.go.dev / module proxy indexing:

   - Visit `https://pkg.go.dev/github.com/yuechen-li-dev/oct` and click **Request**, or
   - Request:
     `https://proxy.golang.org/github.com/yuechen-li-dev/oct/@v/v0.1.0.info`, or
   - Run:

   ```bash
   GOPROXY=https://proxy.golang.org GO111MODULE=on go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
   ```

5. Verify versioned install commands:

   ```bash
   go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0
   go install github.com/yuechen-li-dev/oct/cmd/octxiliary-io@v0.1.0
   ```

Notes:

- `v0` indicates experimental / pre-v1 module status.
- Do not reuse tags.
- If a bad version is published, release a newer version with module retractions rather than mutating an existing tag.

$ErrorActionPreference = "Stop"
go test ./cmd/oct
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go test ./internal/...
exit $LASTEXITCODE

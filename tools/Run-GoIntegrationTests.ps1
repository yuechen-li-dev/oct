param([switch]$IncludeToolchain)

$ErrorActionPreference = "Stop"
go test -tags=integration ./...
if ($LASTEXITCODE -ne 0 -or -not $IncludeToolchain) { exit $LASTEXITCODE }
go test -tags=toolchain ./...
exit $LASTEXITCODE

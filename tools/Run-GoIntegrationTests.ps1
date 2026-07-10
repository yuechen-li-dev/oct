param([switch]$IncludeToolchain)

$ErrorActionPreference = "Stop"
$previousTempRoot = $env:OCT_TEST_TEMP_ROOT
$tempRoot = Join-Path (Get-Location).Path ("out/test-artifacts/oct-test-temp-" + [guid]::NewGuid().ToString("N"))
$env:OCT_TEST_TEMP_ROOT = $tempRoot
$exitCode = 0
try {
    go test -count=1 -tags=integration ./...
    $exitCode = $LASTEXITCODE
    if ($exitCode -eq 0 -and $IncludeToolchain) {
        go test -count=1 -tags=toolchain ./...
        $exitCode = $LASTEXITCODE
    }
} finally {
    if ($env:OCT_KEEP_TEST_ARTIFACTS -eq "1") {
        Write-Output "Retained compiled test artifact root: $tempRoot"
    } elseif (Test-Path -LiteralPath $tempRoot) {
        Remove-Item -LiteralPath $tempRoot -Recurse -Force
    }
    $env:OCT_TEST_TEMP_ROOT = $previousTempRoot
}
exit $exitCode

param(
    [string]$FixtureRoot = (Join-Path (Get-Location).Path 'out/test-artifacts/cleanup-script-fixture')
)

$ErrorActionPreference = 'Stop'
$scriptPath = Join-Path $PSScriptRoot 'Clean-OctTestArtifacts.ps1'
$resolvedFixture = [IO.Path]::GetFullPath($FixtureRoot)
if (-not $resolvedFixture.StartsWith([IO.Path]::GetFullPath((Get-Location).Path), [StringComparison]::OrdinalIgnoreCase)) {
    throw "FixtureRoot must remain inside the current repository: $resolvedFixture"
}

if (Test-Path -LiteralPath $resolvedFixture) {
    Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
}
New-Item -ItemType Directory -Path $resolvedFixture -Force | Out-Null
try {
    $generated = Join-Path $resolvedFixture 'zz_oct_test_runner_123_456.octest.octbin'
    $legitimate = Join-Path $resolvedFixture 'user-build.octbin'
    $unrelated = Join-Path $resolvedFixture 'notes.txt'
    Set-Content -LiteralPath $generated -Value 'generated'
    Set-Content -LiteralPath $legitimate -Value 'legitimate'
    Set-Content -LiteralPath $unrelated -Value 'unrelated'

    $dryRun = & $scriptPath -Root $resolvedFixture -OlderThan ([TimeSpan]::Zero)
    if (-not (Test-Path -LiteralPath $generated)) { throw 'dry-run deleted the generated fixture' }
    if (($dryRun -join "`n") -notmatch [regex]::Escape($generated)) { throw 'dry-run did not list the known generated pattern' }
    if (($dryRun -join "`n") -match [regex]::Escape($legitimate)) { throw 'dry-run matched a legitimate .octbin' }

    & $scriptPath -Root $resolvedFixture -OlderThan ([TimeSpan]::Zero) -Delete | Out-Null
    if (Test-Path -LiteralPath $generated) { throw 'delete mode retained the known generated pattern' }
    if (-not (Test-Path -LiteralPath $legitimate)) { throw 'delete mode removed a legitimate .octbin' }
    if (-not (Test-Path -LiteralPath $unrelated)) { throw 'delete mode removed an unrelated file' }
    Write-Output 'PASS Clean-OctTestArtifacts narrowly matches generated compiled-test artifacts.'
} finally {
    if (Test-Path -LiteralPath $resolvedFixture) {
        Remove-Item -LiteralPath $resolvedFixture -Recurse -Force
    }
}

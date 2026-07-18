$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$sourceRoot = Join-Path $PSScriptRoot "fixtures/JudgeDemo"
$fixtureRoot = Join-Path $PSScriptRoot "fixtures"
$stageRoot = Join-Path $repoRoot ".tmp/build-week-recording/RepairDemo"
Set-Location $repoRoot

New-Item -ItemType Directory -Force -Path $stageRoot | Out-Null
Copy-Item -Force (Join-Path $sourceRoot "manifest.oct") (Join-Path $stageRoot "manifest.oct")
Copy-Item -Force (Join-Path $sourceRoot "JudgeDemo.oct") (Join-Path $stageRoot "JudgeDemo.oct")
Copy-Item -Force (Join-Path $fixtureRoot "repair-before.octest.txt") (Join-Path $stageRoot "JudgeDemo.octest")

function Invoke-OctTest {
    $relativeStage = ".tmp/build-week-recording/RepairDemo"
    $savedPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    $output = & go run ./cmd/oct test $relativeStage --execution auto --json 2>&1
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = $savedPreference
    $output | ForEach-Object { Write-Host $_ }
    return $exitCode
}

Write-Host "BEFORE - normalized compiler diagnostic" -ForegroundColor Yellow
$beforeExit = Invoke-OctTest
if ($beforeExit -eq 0) {
    throw "The intentionally invalid fixture unexpectedly passed"
}

Write-Host ""
Write-Host "REPAIR - replace String argument with Int argument" -ForegroundColor Cyan
Copy-Item -Force (Join-Path $fixtureRoot "repair-after.octest.txt") (Join-Path $stageRoot "JudgeDemo.octest")

Write-Host "AFTER - rerun the same canonical command" -ForegroundColor Green
$afterExit = Invoke-OctTest
if ($afterExit -ne 0) {
    throw "The repaired fixture failed with exit code $afterExit"
}

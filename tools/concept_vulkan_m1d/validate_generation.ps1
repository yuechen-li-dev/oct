param()

$ErrorActionPreference = 'Stop'
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$tmpRoot = Join-Path $repoRoot 'out\prometheus\native\concept_vulkan_m1d_generation'
$a = Join-Path $tmpRoot 'a'
$b = Join-Path $tmpRoot 'b'
$stale = Join-Path $tmpRoot 'stale'

Remove-Item -Recurse -Force $tmpRoot -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $a,$b,$stale | Out-Null

go run ./cmd/concept-vulkan -out $a generate
go run ./cmd/concept-vulkan -out $b generate

$aFiles = Get-ChildItem $a -File | Sort-Object Name
$bFiles = Get-ChildItem $b -File | Sort-Object Name
if ((($aFiles.Name) -join ',') -ne (($bFiles.Name) -join ',')) {
    throw 'deterministic generation failed: file sets differ'
}
foreach ($f in $aFiles) {
    $other = Join-Path $b $f.Name
    $left = (Get-FileHash $f.FullName -Algorithm SHA256).Hash
    $right = (Get-FileHash $other -Algorithm SHA256).Hash
    if ($left -ne $right) {
        throw "deterministic generation failed: $($f.Name) differs"
    }
}

go run ./cmd/concept-vulkan -out $stale generate
Add-Content -Path (Join-Path $stale 'reactor_concept_vulkan_kernel54.generated.c') -Value '/* stale mutation */'
$oldNative = $PSNativeCommandUseErrorActionPreference
$PSNativeCommandUseErrorActionPreference = $false
$staleOutput = & cmd.exe /c "go run ./cmd/concept-vulkan -out `"$stale`" check 2>&1"
$PSNativeCommandUseErrorActionPreference = $oldNative
if ($LASTEXITCODE -eq 0) {
    throw 'stale-output rejection failed: check unexpectedly passed'
}
if (($staleOutput -join "`n") -notmatch 'CV3001') {
    throw 'stale-output rejection failed: CV3001 was not reported'
}

Write-Host 'PASS: deterministic double generation'
Write-Host 'PASS: stale-output rejection'

param(
    [string]$Root = (Get-Location).Path,
    [TimeSpan]$OlderThan = ([TimeSpan]::FromHours(1)),
    [switch]$Delete,
    [switch]$IncludeTempDirectories
)

$ErrorActionPreference = "Stop"
$resolvedRoot = (Resolve-Path -LiteralPath $Root).Path
$cutoff = (Get-Date).Subtract($OlderThan)
$filePattern = 'zz_oct_test_runner_*.octest.octbin'
$directoryPatterns = @('octest-run-*', 'oct-artifact-run-*', 'oct-benchmark-run-*')

$fileMatches = @(Get-ChildItem -LiteralPath $resolvedRoot -Recurse -File -Filter $filePattern -ErrorAction Stop |
    Where-Object { $_.LastWriteTime -le $cutoff })

$directoryMatches = @()
if ($IncludeTempDirectories) {
    $directoryMatches = @(Get-ChildItem -LiteralPath $resolvedRoot -Recurse -Directory -ErrorAction Stop |
        Where-Object {
            $name = $_.Name
            ($directoryPatterns | Where-Object { $name -like $_ }).Count -gt 0 -and
            $_.LastWriteTime -le $cutoff
        })
}

$directoryFiles = @($directoryMatches | ForEach-Object {
    Get-ChildItem -LiteralPath $_.FullName -Recurse -File -ErrorAction Stop
})
$bytes = [int64](($fileMatches + $directoryFiles | Measure-Object -Property Length -Sum).Sum)

$mode = if ($Delete) { 'DELETE' } else { 'DRY-RUN' }
Write-Output "$mode Oct compiled-test artifact cleanup"
Write-Output "Root: $resolvedRoot"
Write-Output "Older than: $OlderThan (cutoff $($cutoff.ToString('o')))"
foreach ($match in $fileMatches) {
    Write-Output ("FILE {0} ({1} bytes)" -f $match.FullName, $match.Length)
}
foreach ($match in $directoryMatches) {
    Write-Output "DIR  $($match.FullName)"
}
Write-Output ("Matched: {0} file(s), {1} directories, {2} bytes" -f $fileMatches.Count, $directoryMatches.Count, $bytes)

if (-not $Delete) {
    Write-Output 'No files were deleted. Pass -Delete to remove only the listed matches.'
    return
}

foreach ($match in $fileMatches) {
    Remove-Item -LiteralPath $match.FullName -Force -ErrorAction Stop
}
# Delete deepest scopes first if callers explicitly include lifecycle directories.
foreach ($match in ($directoryMatches | Sort-Object { $_.FullName.Length } -Descending)) {
    if (Test-Path -LiteralPath $match.FullName) {
        Remove-Item -LiteralPath $match.FullName -Recurse -Force -ErrorAction Stop
    }
}
Write-Output ("Deleted: {0} file(s), {1} directories; reclaimed {2} bytes" -f $fileMatches.Count, $directoryMatches.Count, $bytes)

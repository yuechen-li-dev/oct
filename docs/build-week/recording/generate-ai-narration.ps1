param(
    [string]$Voice = "en-US-AndrewMultilingualNeural",
    [ValidateRange(-50, 100)]
    [int]$RatePercent = 30,
    [string]$OutputPath = "out/build-week-video/narration-neural-fast.mp3"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
$scriptPath = Join-Path $repoRoot "docs/build-week/VIDEO_SCRIPT.md"
$absoluteOutput = Join-Path $repoRoot $OutputPath
$outputDirectory = Split-Path -Parent $absoluteOutput
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

$videoScript = Get-Content -LiteralPath $scriptPath -Raw
$narration = ((($videoScript -split "## Exact narration")[1]) -split "## Title card")[0]
$narration = [regex]::Replace($narration, "(?m)^###.*$", "")
$narration = $narration.Replace('`', '')
$paragraphs = @($narration -split "(?:\r?\n){2,}" | ForEach-Object { ([regex]::Replace($_, "\s+", " ")).Trim() } | Where-Object { $_ -ne "" })
$plainText = $paragraphs -join "`r`n`r`n"
$plainPath = [System.IO.Path]::ChangeExtension($absoluteOutput, ".txt")
[System.IO.File]::WriteAllText($plainPath, $plainText, [System.Text.UTF8Encoding]::new($false))

$runtime = Join-Path $repoRoot ".tmp/edge-tts-runtime"
$edgePackage = Join-Path $runtime "edge_tts"
if (-not (Test-Path -LiteralPath $edgePackage)) {
    Write-Host "Installing the bounded narration dependency under .tmp/edge-tts-runtime..."
    & python -m pip install --target $runtime "edge-tts==7.2.8" --disable-pip-version-check
    if ($LASTEXITCODE -ne 0) {
        throw "Could not install edge-tts into the repository-local runtime."
    }
}

$oldPythonPath = $env:PYTHONPATH
try {
    $env:PYTHONPATH = if ($oldPythonPath) { "$runtime;$oldPythonPath" } else { $runtime }
    $subtitlePath = [System.IO.Path]::ChangeExtension($absoluteOutput, ".vtt")
    $rate = if ($RatePercent -ge 0) { "+$RatePercent%" } else { "$RatePercent%" }
    & python -m edge_tts --voice $Voice --rate $rate --file $plainPath --write-media $absoluteOutput --write-subtitles $subtitlePath
    if ($LASTEXITCODE -ne 0) {
        throw "Neural narration generation failed."
    }
} finally {
    $env:PYTHONPATH = $oldPythonPath
}

Write-Host "Narration voice: $Voice (neural)"
Write-Host "Narration rate: $RatePercent%"
Write-Host "Narration media: $absoluteOutput"
Write-Host "Narration text: $plainPath"
Write-Host "Narration timing: $subtitlePath"

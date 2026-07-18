$ErrorActionPreference = "Stop"
$packetRoot = $PSScriptRoot
$repoRoot = (Resolve-Path (Join-Path $packetRoot "../..")).Path
Set-Location $repoRoot

function Assert-True([bool]$Condition, [string]$Message) {
    if (-not $Condition) {
        throw $Message
    }
}

$required = @(
    "README.md",
    "SUBMISSION.md",
    "BUILD_WEEK_SCOPE.md",
    "CODEX_COLLABORATION.md",
    "JUDGE_QUICKSTART.md",
    "TESTING_INSTRUCTIONS.md",
    "EVIDENCE_INDEX.md",
    "DEVPOST_FIELDS.md",
    "VIDEO_SCRIPT.md",
    "VIDEO_SHOT_LIST.md",
    "VIDEO_CAPTIONS.srt",
    "BUILD_WEEK_COMMITS.json",
    "recording/README.md"
)
foreach ($relative in $required) {
    Assert-True (Test-Path -LiteralPath (Join-Path $packetRoot $relative)) "Missing packet file: $relative"
}

# Build Week commit dates are ISO-8601 strings, not host-local DateTime values.
# Without DateKind String PowerShell coerces the JSON value and formats it using
# the machine locale before the exact Git comparison below.
$ledger = Get-Content (Join-Path $packetRoot "BUILD_WEEK_COMMITS.json") -Raw | ConvertFrom-Json -DateKind String
Assert-True ($ledger.schemaVersion -eq "oct.build-week.commits.v1") "Unexpected ledger schema"
Assert-True ($ledger.commits.Count -eq 23) "Expected 23 eligible commits"
$uniqueShas = @($ledger.commits.sha | Sort-Object -Unique)
Assert-True ($uniqueShas.Count -eq 23) "Commit ledger contains duplicate SHAs"

foreach ($entry in $ledger.commits) {
    $actualDate = (git show -s --format=%aI $entry.sha).Trim()
    Assert-True ($LASTEXITCODE -eq 0) "Unknown commit: $($entry.sha)"
    Assert-True ($actualDate -eq $entry.date) "Date mismatch for $($entry.sha): $actualDate != $($entry.date)"
    Assert-True ($entry.date -ge $ledger.eligibilityStart) "Pre-boundary commit in eligible ledger: $($entry.sha)"
    foreach ($path in @($entry.representativeFiles + $entry.tests + $entry.artifacts)) {
        if (($path -notmatch "^(M48 |M48 per-layer|M48 stage-level)") -and ($path -notmatch "^internal/prometheus/shaders/sdslv/production/sgemm$") -and ($path -notmatch "^Examples/SDSL-V/conformance/invalid$")) {
            Assert-True (Test-Path -LiteralPath $path) "Missing ledger evidence path: $path"
        }
    }
}

$markdownFiles = @(
    (Join-Path $repoRoot "README.md")
) + @(Get-ChildItem -LiteralPath $packetRoot -Filter *.md -File | Select-Object -ExpandProperty FullName)

foreach ($file in $markdownFiles) {
    $text = Get-Content -LiteralPath $file -Raw
    $links = [regex]::Matches($text, "\[[^\]]+\]\(([^)]+)\)")
    foreach ($match in $links) {
        $target = $match.Groups[1].Value.Trim("<", ">")
        if (($target -match "^(https?://|mailto:|#)") -or ($target -eq "")) {
            continue
        }
        $target = ($target -split "#")[0]
        $resolved = Join-Path (Split-Path -Parent $file) $target
        Assert-True (Test-Path -LiteralPath $resolved) "Broken local link in $file : $target"
    }
}

$srt = Get-Content (Join-Path $packetRoot "VIDEO_CAPTIONS.srt") -Raw
$timeMatches = [regex]::Matches($srt, "(?m)^(\d{2}):(\d{2}):(\d{2}),(\d{3}) --> (\d{2}):(\d{2}):(\d{2}),(\d{3})$")
Assert-True ($timeMatches.Count -gt 0) "No SRT time ranges found"
$previousEnd = 0.0
foreach ($time in $timeMatches) {
    $start = ([int]$time.Groups[1].Value * 3600) + ([int]$time.Groups[2].Value * 60) + [int]$time.Groups[3].Value + ([int]$time.Groups[4].Value / 1000.0)
    $end = ([int]$time.Groups[5].Value * 3600) + ([int]$time.Groups[6].Value * 60) + [int]$time.Groups[7].Value + ([int]$time.Groups[8].Value / 1000.0)
    Assert-True ($start -ge $previousEnd) "SRT ranges overlap or go backward"
    Assert-True ($end -gt $start) "SRT range has non-positive duration"
    $previousEnd = $end
}
Assert-True ($previousEnd -eq 164.0) "Expected subtitle end at 00:02:44, got $previousEnd seconds"

$video = Get-Content (Join-Path $packetRoot "VIDEO_SCRIPT.md") -Raw
$narration = ((($video -split "## Exact narration")[1]) -split "## Title card")[0]
$narration = [regex]::Replace($narration, "(?m)^###.*$", "")
$captionText = [regex]::Replace($srt, "(?m)^\d+$", "")
$captionText = [regex]::Replace($captionText, "(?m)^\d{2}:.*$", "")
$narrationTokens = [regex]::Matches($narration.ToLowerInvariant(), "[a-z0-9]+") | ForEach-Object { $_.Value }
$captionTokens = [regex]::Matches($captionText.ToLowerInvariant(), "[a-z0-9]+") | ForEach-Object { $_.Value }
Assert-True (($narrationTokens -join " ") -eq ($captionTokens -join " ")) "Subtitle text does not exactly match narration"
$wordCount = ($narration -split "\s+" | Where-Object { $_ -match "[A-Za-z0-9]" }).Count
$wpm = [math]::Round(($wordCount / 164.0) * 60.0, 1)
Assert-True (($wpm -ge 130.0) -and ($wpm -le 155.0)) "Narration rate is outside 130-155 WPM: $wpm"

$devpost = Get-Content (Join-Path $packetRoot "DEVPOST_FIELDS.md") -Raw
$devpostSections = [regex]::Matches($devpost, "(?ms)^## (?<name>[^\r\n]+)\r?\n\r?\n(?<body>.*?)(?=^## |\z)")
$fieldLimits = @{ "Project name" = 80; "Tagline" = 200; "Short description" = 500; "Long description" = 2000 }
foreach ($section in $devpostSections) {
    $fieldName = $section.Groups["name"].Value
    if ($fieldLimits.ContainsKey($fieldName)) {
        $fieldLength = $section.Groups["body"].Value.Trim().Length
        Assert-True ($fieldLength -le $fieldLimits[$fieldName]) "Devpost field exceeds packet cap: $fieldName is $fieldLength characters"
    }
}

$allPacketText = (Get-ChildItem -LiteralPath $packetRoot -Recurse -File | Where-Object { ($_.Extension -in @(".md", ".srt", ".json", ".ps1", ".html", ".txt", ".oct", ".octest")) -and ($_.Name -ne "validate-packet.ps1") } | ForEach-Object { Get-Content -LiteralPath $_.FullName -Raw }) -join "`n"
Assert-True ($allPacketText -notmatch "C:\\Users\\|/Users/|/home/") "Private absolute path found in packet"
$placeholders = [regex]::Matches($allPacketText, "\[VIDEO_URL_OWNER_REQUIRED\]")
$otherPlaceholders = [regex]::Matches($allPacketText, "\[[A-Z][A-Z0-9_]*OWNER_REQUIRED\]") | Where-Object { $_.Value -ne "[VIDEO_URL_OWNER_REQUIRED]" }
Assert-True ($otherPlaceholders.Count -eq 0) "Unexpected owner placeholder found"
Assert-True ($placeholders.Count -ge 1) "Required video URL placeholder is missing"
Assert-True ($allPacketText -match "019f6cb4-b438-70e2-b91c-487d7ad45bbd") "Confirmed feedback Session ID is missing"

$licenseText = Get-Content (Join-Path $repoRoot "LICENSE") -Raw
Assert-True ($licenseText -match "GNU GENERAL PUBLIC LICENSE") "Public license check failed"
$marketplace = Get-Content (Join-Path $repoRoot ".agents/plugins/marketplace.json") -Raw | ConvertFrom-Json
Assert-True ($marketplace.plugins[0].category -eq "Developer Tools") "Plugin marketplace category mismatch"

Write-Host "PACKET VALIDATION PASSED" -ForegroundColor Green
Write-Host "Eligible commits: 23"
Write-Host "Narration: $wordCount words, 164 seconds, $wpm WPM"
Write-Host "Devpost caps: title <= 80, tagline <= 200, short <= 500, long <= 2000"
Write-Host "Owner placeholders: VIDEO URL only; /feedback Session ID confirmed"

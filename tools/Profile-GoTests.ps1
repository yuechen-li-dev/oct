param(
    [Parameter(Mandatory = $true)]
    [string[]]$JsonPath,
    [Parameter(Mandatory = $true)]
    [string]$OutputJson,
    [Parameter(Mandatory = $true)]
    [string]$OutputMarkdown,
    [string]$Label = "Go test latency"
)

$ErrorActionPreference = "Stop"

$testFiles = Get-ChildItem cmd/oct, internal -Recurse -Filter *_test.go
$testToFile = @{}
foreach ($file in $testFiles) {
    $text = Get-Content $file.FullName -Raw
    foreach ($match in [regex]::Matches($text, '(?m)^func (Test\w+)\(')) {
        $testToFile[$match.Groups[1].Value] = $file.FullName.Substring((Get-Location).Path.Length + 1)
    }
}

$runs = @()
$events = foreach ($path in $JsonPath) {
    $lane = if ($path -match 'integration') { 'integration' } elseif ($path -match 'toolchain') { 'toolchain' } else { 'default' }
    $wallPath = Join-Path (Split-Path -Parent $path) (([IO.Path]::GetFileNameWithoutExtension($path)) + '_wall.json')
    if (Test-Path -LiteralPath $wallPath) {
        $wall = Get-Content -LiteralPath $wallPath -Raw | ConvertFrom-Json
        $runs += [pscustomobject]@{
            lane = $lane
            input = $path
            wall_seconds = [double]$wall.wall_seconds
            exit_code = [int]$wall.exit_code
            owned_temp_entries = $wall.owned_temp_entries
            legacy_leaks = $wall.legacy_leaks
        }
    }
    Get-Content $path | ForEach-Object {
        try {
            $event = $_ | ConvertFrom-Json
            $event | Add-Member -NotePropertyName _lane -NotePropertyValue $lane
            $event
        } catch { }
    }
}

$packages = @($events |
    Where-Object { $_.Action -eq 'pass' -and -not $_.Test -and $null -ne $_.Elapsed } |
    ForEach-Object { [pscustomobject]@{ lane = $_._lane; package = $_.Package; seconds = [double]$_.Elapsed } } |
    Sort-Object seconds -Descending)

$tests = @($events |
    Where-Object { $_.Action -eq 'pass' -and $_.Test -and $null -ne $_.Elapsed } |
    ForEach-Object {
        $top = ($_.Test -split '/')[0]
        [pscustomobject]@{
            lane = $_._lane
            package = $_.Package
            test = $_.Test
            file = $testToFile[$top]
            seconds = [double]$_.Elapsed
        }
    } |
    Sort-Object seconds -Descending)

$families = @($tests |
    Where-Object { $_.test -notmatch '/' } |
    ForEach-Object {
        $name = $_.test -replace '^Test', ''
        $family = if ($name -match '^([A-Z][a-z]+(?:[A-Z][a-z]+)?)') { $Matches[1] } else { $name }
        [pscustomobject]@{ family = $family; seconds = $_.seconds }
    } |
    Group-Object family |
    ForEach-Object {
        [pscustomobject]@{
            family = $_.Name
            seconds = [math]::Round(($_.Group | Measure-Object seconds -Sum).Sum, 3)
            count = $_.Count
        }
    } |
    Sort-Object seconds -Descending)

$patterns = [ordered]@{
    subprocess_sites = 'exec\.Command(?:Context)?\('
    go_run_sites = 'exec\.Command\("go",\s*"run"'
    external_tool_sites = 'exec\.(?:Command|LookPath).*?(?:git|go|node|dxc|vulkan|clang|gcc|powershell)'
    tree_walk_or_copy_sites = 'WalkDir|filepath\.Walk|copyDir\('
    sleep_sites = 'time\.Sleep\('
    cwd_mutation_sites = 'os\.Chdir\('
    environment_mutation_sites = 't\.Setenv\(|os\.Setenv\('
    shared_setup_sites = 'sync\.Once|func TestMain\('
}
$static = [ordered]@{}
foreach ($entry in $patterns.GetEnumerator()) {
    $matches = @($testFiles | Select-String -Pattern $entry.Value)
    $static[$entry.Key] = [pscustomobject]@{
        count = $matches.Count
        locations = @($matches | ForEach-Object { "$($_.Path.Substring((Get-Location).Path.Length + 1)):$($_.LineNumber)" })
    }
}

$summary = [ordered]@{
    label = $Label
    generated_at = (Get-Date).ToString('o')
    inputs = $JsonPath
    runs = $runs
    packages = $packages
    slowest_tests = @($tests | Select-Object -First 50)
    families = @($families | Select-Object -First 30)
    static_source_audit_at_summary_time = $static
}

$jsonDir = Split-Path -Parent $OutputJson
$mdDir = Split-Path -Parent $OutputMarkdown
if ($jsonDir) { New-Item -ItemType Directory -Force $jsonDir | Out-Null }
if ($mdDir) { New-Item -ItemType Directory -Force $mdDir | Out-Null }
$summary | ConvertTo-Json -Depth 8 | Set-Content -Encoding utf8 $OutputJson

$lines = @("# $Label", "", "Generated from Go's `-json` event stream.", "", "## Lane wall time", "", "| Lane | Wall seconds | Exit | Owned temp entries | Legacy leaks |", "|---|---:|---:|---:|---:|")
foreach ($run in $runs) { $lines += "| $($run.lane) | $($run.wall_seconds.ToString('0.000')) | $($run.exit_code) | $($run.owned_temp_entries) | $($run.legacy_leaks) |" }
$lines += @("", "## Package durations", "", "| Lane | Package | Seconds |", "|---|---|---:|")
foreach ($row in $packages) { $lines += "| $($row.lane) | $($row.package) | $($row.seconds.ToString('0.000')) |" }
$lines += @("", "## Slowest tests", "", "| Lane | Test | File | Seconds |", "|---|---|---|---:|")
foreach ($row in ($tests | Select-Object -First 30)) { $lines += "| $($row.lane) | $($row.test) | $($row.file) | $($row.seconds.ToString('0.000')) |" }
$lines += @("", "## Highest aggregate top-level families", "", "| Family | Tests | Seconds |", "|---|---:|---:|")
foreach ($row in ($families | Select-Object -First 20)) { $lines += "| $($row.family) | $($row.count) | $($row.seconds.ToString('0.000')) |" }
$lines += @("", "## Static heavyweight-pattern audit at summary time", "", "This source scan reflects the checkout when the summary is generated; raw Go JSON inputs remain the timing authority.", "", "| Pattern | Sites |", "|---|---:|")
foreach ($entry in $static.GetEnumerator()) { $lines += "| $($entry.Key) | $($entry.Value.count) |" }
$lines | Set-Content -Encoding utf8 $OutputMarkdown

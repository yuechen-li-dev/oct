param(
    [Parameter(Mandatory = $true)] [string] $Version,
    [Parameter(Mandatory = $true)] [string] $OutputDirectory
)

$ErrorActionPreference = 'Stop'
$repo = (Resolve-Path (Join-Path $PSScriptRoot '..')).Path
if ($Version -notmatch '^(\d+)\.(\d+)\.') { throw "release version must begin with major.minor.patch: $Version" }
$installGuide = Join-Path $repo ("docs\releases\INSTALL_{0}_{1}.md" -f $Matches[1], $Matches[2])
if (-not (Test-Path -LiteralPath $installGuide)) { throw "missing release install guide: $installGuide" }
$out = [System.IO.Path]::GetFullPath($OutputDirectory)
$name = "oct-$Version-windows-amd64"
$stage = Join-Path $out "stage-$name"
$root = Join-Path $stage $name
$archive = Join-Path $out "$name.zip"

New-Item -ItemType Directory -Force -Path $out | Out-Null
if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
if (Test-Path -LiteralPath $archive) { Remove-Item -LiteralPath $archive -Force }
New-Item -ItemType Directory -Force -Path $root, (Join-Path $root 'runtime\internal\octxiliary'), (Join-Path $root 'runtime\internal\dimension'), (Join-Path $root 'sidecars') | Out-Null

Push-Location $repo
try {
    go build -trimpath -ldflags "-X github.com/yuechen-li-dev/oct/internal/cli.version=$Version" -o (Join-Path $root 'oct.exe') ./cmd/oct
    if ($LASTEXITCODE -ne 0) { throw "Oct CLI build failed with exit code $LASTEXITCODE" }
    go run ./tools/build_sidecars --out (Join-Path $root 'sidecars')
    if ($LASTEXITCODE -ne 0) { throw "Octxiliary sidecar build failed with exit code $LASTEXITCODE" }
} finally { Pop-Location }

Copy-Item (Join-Path $repo 'LICENSE') (Join-Path $root 'LICENSE')
Copy-Item $installGuide (Join-Path $root 'INSTALL.md')
Copy-Item (Join-Path $repo 'go.mod') (Join-Path $root 'runtime\go.mod')
Copy-Item (Join-Path $repo 'go.sum') (Join-Path $root 'runtime\go.sum')
Get-ChildItem (Join-Path $repo 'internal\octxiliary') -Filter '*.go' | Where-Object { $_.Name -notlike '*_test.go' } | Copy-Item -Destination (Join-Path $root 'runtime\internal\octxiliary')
Get-ChildItem (Join-Path $repo 'internal\dimension') -Filter '*.go' | Where-Object { $_.Name -notlike '*_test.go' } | Copy-Item -Destination (Join-Path $root 'runtime\internal\dimension')
Compress-Archive -Path $root -DestinationPath $archive -CompressionLevel Optimal
Get-ChildItem -LiteralPath $out -File | Where-Object { $_.Name -match '^oct-.*\.(zip|tar\.gz)$' } | Sort-Object Name | ForEach-Object {
    "{0} *{1}" -f (Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLowerInvariant(), $_.Name
} | Set-Content -LiteralPath (Join-Path $out 'checksums.sha256') -NoNewline:$false
Write-Output $archive

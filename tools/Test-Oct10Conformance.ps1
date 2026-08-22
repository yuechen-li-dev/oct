param(
    [string]$OctPath = "",
    [ValidateSet("interpreted", "compiled")]
    [string]$Execution = "compiled"
)

$ErrorActionPreference = "Stop"
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($OctPath -eq "") {
    $OctPath = Join-Path $repoRoot "oct.exe"
}
if (-not (Test-Path -LiteralPath $OctPath -PathType Leaf)) {
    throw "Oct executable not found: $OctPath. Build one first, for example: go build -o .tmp\\oct-rc2.exe ./cmd/oct"
}

# This is the Oct 1.0 positive conformance inventory. Each entry is a selected
# source file, rather than its containing corpus directory, so package roots
# with sibling imports resolve through the same public CLI path users invoke.
$stableSources = @(
    "Language/Builtins/FloatClamp01/valid/float_clamp01_compiled.octest",
    "Language/Concurrency/Batch/valid/batch_valid.octest",
    "Language/ControlFlow/ConditionSwitch/valid/condition_switch.octest",
    "Language/ControlFlow/Loops/valid/for_loop_descend_sums.octest",
    "Language/ControlFlow/OctomataCompiledBoundary/valid/compiled_boundary_core.octest",
    "Language/Expressions/UtilityWhen/valid/standalone_utility_when.octest",
    "Language/Functions/FunctionValues/valid/function_values.octest",
    "Language/Packages/CrossPackageM81/valid/Main/cross_package_m81.octest",
    "Language/Testing/CompiledBuiltinSweep/valid/core_pure_builtins.octest",
    "Language/Types/Arrays/valid/array_cross_section.octest",
    "Language/Types/ComplexM0/valid/complex_core_surface.octest",
    "Language/Types/EnumsAssociated/valid/associated_data_enums_match_binding.octest",
    "Language/Types/UnitsM1/valid/signed_exponents_and_hz_alias_m1.octest",
    "Language/Types/VectorsMatricesM92/valid/dimensioned_linear_algebra_m92.octest",
    "Examples/SmartGreenhouseController/SmartGreenhouseController.octest"
)

$compiled = 0
$fallback = 0
$failed = 0
foreach ($relativeSource in $stableSources) {
    $source = Join-Path $repoRoot $relativeSource
    if (-not (Test-Path -LiteralPath $source -PathType Leaf)) {
        throw "Stable conformance source is missing: $relativeSource"
    }
    $output = & $OctPath test $source --execution $Execution 2>&1
    $exitCode = $LASTEXITCODE
    $text = ($output | Out-String)
    $summary = [regex]::Match($text, "Execution summary: compiled: (\d+) interpreted fallback: (\d+)")
    if ($exitCode -ne 0 -or -not $summary.Success) {
        Write-Error "FAIL $relativeSource`n$text"
        $failed++
        continue
    }
    $caseCompiled = [int]$summary.Groups[1].Value
    $caseFallback = [int]$summary.Groups[2].Value
    if ($Execution -eq "compiled" -and ($caseCompiled -eq 0 -or $caseFallback -ne 0)) {
        Write-Error "FAIL ${relativeSource}: compiled conformance requires native cases and zero fallback`n$text"
        $failed++
        continue
    }
    $compiled += $caseCompiled
    $fallback += $caseFallback
    Write-Host "PASS $relativeSource"
}

Write-Host "Oct 1.0 $Execution conformance: targets=$($stableSources.Count) compiled=$compiled fallback=$fallback failed=$failed"
if ($failed -ne 0) {
    exit 1
}
if ($Execution -eq "compiled" -and $compiled -eq 0) {
    throw "Compiled conformance discovered zero native cases"
}
if ($Execution -eq "compiled" -and $fallback -ne 0) {
    throw "Compiled conformance observed interpreter fallback"
}

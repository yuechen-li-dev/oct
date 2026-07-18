$ErrorActionPreference = "Stop"

$rows = @(
    [pscustomobject]@{ Milestone = "M42"; SHA = "0392276"; Capability = "Device-resident attention" }
    [pscustomobject]@{ Milestone = "M43"; SHA = "a5f0fd8"; Capability = "Bounded grouped multi-head attention" }
    [pscustomobject]@{ Milestone = "M44"; SHA = "7de0809"; Capability = "Aggregation + output projection" }
    [pscustomobject]@{ Milestone = "M45"; SHA = "1032853"; Capability = "Residual add" }
    [pscustomobject]@{ Milestone = "M46"; SHA = "2ecd200"; Capability = "RMSNorm" }
    [pscustomobject]@{ Milestone = "M47"; SHA = "9fb3772"; Capability = "Gated FFN + complete bounded block" }
    [pscustomobject]@{ Milestone = "M48"; SHA = "00eab1b"; Capability = "Fixed four-block stack + numerical audit" }
    [pscustomobject]@{ Milestone = "M49"; SHA = "a1ab67a"; Capability = "Numerical heterogeneity research in Oct" }
    [pscustomobject]@{ Milestone = "M49a"; SHA = "be4bfd1"; Capability = "Controlled checkpoint mitigation" }
    [pscustomobject]@{ Milestone = "M49b"; SHA = "51b08bf"; Capability = "Experimental Shadow-HSFM observer" }
)

Write-Host "PROMETHEUS BUILD WEEK VERTICAL" -ForegroundColor Cyan
$rows | Format-Table -AutoSize
Write-Host "Scope: fixed experimental topology; live witness: Windows RTX 3070" -ForegroundColor Yellow
Write-Host "Not claimed: general LLM runtime, CUDA superiority, or cross-vendor DVT"

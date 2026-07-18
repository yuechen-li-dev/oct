param(
    [string]$OutputPath = "out/build-week-video/presentation.html"
)

$ErrorActionPreference = "Stop"
$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "../../..")).Path
Set-Location $repoRoot
$absoluteOutput = Join-Path $repoRoot $OutputPath
New-Item -ItemType Directory -Force -Path (Split-Path -Parent $absoluteOutput) | Out-Null

function Get-Lines([string]$Path, [int]$Start, [int]$Count) {
    return ((Get-Content -LiteralPath (Join-Path $repoRoot $Path) | Select-Object -Skip $Start -First $Count) -join "`n")
}

function Invoke-OctCapture([string[]]$Arguments) {
    $octPath = Join-Path $repoRoot ".tmp/judge-workflow/oct.exe"
    if (Test-Path -LiteralPath $octPath) {
        $result = & $octPath @Arguments 2>&1
    } else {
        $result = & go run ./cmd/oct @Arguments 2>&1
    }
    if ($LASTEXITCODE -ne 0) {
        throw "Oct capture failed: $($Arguments -join ' ')"
    }
    return ($result | Out-String).Trim()
}

$timeline = @(
    "ELIGIBILITY BOUNDARY  2026-07-13 09:00 Pacific",
    "PRE-EVENT             8c029d6d  2026-07-12  Oct/SDSL-V/Prometheus foundation",
    "",
    (git log --reverse --date=format-local:"%m-%d %H:%M" --format="%h  %ad  %s" 8c029d6d8f8d5f698276edfda138fa96f5fb305e..b07c8849efa00fe0455e827e9a162856f389878f | Select-Object -First 12),
    "...",
    "b07c8849  07-17 12:51  Codex-native plugin + structured CLI dogfood"
) -join "`n"

$ladder = @(
    "M42  0392276  Device-resident attention",
    "M43  a5f0fd8  Bounded grouped multi-head attention",
    "M44  7de0809  Aggregation + output projection",
    "M45  1032853  Residual add",
    "M46  2ecd200  RMSNorm",
    "M47  9fb3772  Gated FFN + COMPLETE BOUNDED BLOCK",
    "M48  00eab1b  Fixed four-block stack + numerical audit",
    "M49  a1ab67a  Numerical heterogeneity research in Oct",
    "M49a be4bfd1  Controlled checkpoint mitigation",
    "M49b 51b08bf  Experimental Shadow-HSFM observer"
) -join "`n"

$m47 = Get-Content -LiteralPath "internal/prometheus/DevelopmentReport/artifacts/M47/gated_ffn_complete_transformer_block_rtx3070.json" -Raw | ConvertFrom-Json
$m48 = Get-Content -LiteralPath "internal/prometheus/DevelopmentReport/artifacts/M48/multi_block_golden_path_evt_closeout.json" -Raw | ConvertFrom-Json
$hardware = @(
    "M47 COMPLETE BOUNDED BLOCK",
    "schema: $($m47.schema)",
    "validation: warnings=$($m47.validation.warnings) errors=$($m47.validation.errors)",
    "recorded strategies: $($m47.records.Count)",
    "",
    "M48 FIXED FOUR-BLOCK STACK",
    "outcome: $($m48.convergence_outcome)",
    "device: $($m48.provenance.device)",
    "host: $($m48.provenance.host_os) / $($m48.provenance.compiler)",
    "validation errors: $($m48.provenance.validation_errors)",
    "layers: $($m48.shape.primary.layers)  tokens: $($m48.shape.primary.tokens)  width: $($m48.shape.primary.model_width)",
    "intermediate host copies: $($m48.plan.intermediate_host_copy_count)",
    "precision audit passed: $($m48.primary_precision_audit.passed)",
    "preserved failure: layer $($m48.primary_precision_audit.failure.layer), stage $($m48.primary_precision_audit.failure.stage), abs error $($m48.primary_precision_audit.failure.absolute_error)"
) -join "`n"

$testOutput = Invoke-OctCapture @("test", "docs/build-week/recording/fixtures/JudgeDemo", "--execution", "auto", "--json")
$artifactOutput = Invoke-OctCapture @("artifact", "docs/build-week/recording/fixtures/JudgeDemo", "--execution", "interpreted", "--json")
$repairOutput = (& powershell -NoProfile -File "docs/build-week/recording/run-repair-demo.ps1" 2>&1 | Out-String).Trim()

# Captured CLI evidence contains the local checkout path. Keep the recording
# portable and avoid publishing an owner-specific filesystem location.
$pathForms = @(
    $repoRoot,
    $repoRoot.Replace("\", "/"),
    $repoRoot.Replace("\", "\\")
)
foreach ($pathForm in $pathForms) {
    $testOutput = $testOutput.Replace($pathForm, "<repo>")
    $artifactOutput = $artifactOutput.Replace($pathForm, "<repo>")
    $repairOutput = $repairOutput.Replace($pathForm, "<repo>")
}

$slides = @(
    [ordered]@{ start = 0; title = "OCT + SDSL-V"; kicker = "OPENAI BUILD WEEK 2026 / DEVELOPER TOOLS"; kind = "hero"; body = "PROGRAMMING LANGUAGES BUILT BY AI, USED BY AI"; note = "github.com/yuechen-li-dev/oct" },
    [ordered]@{ start = 7; title = "Three layers, one evidence loop"; kicker = "PRE-EXISTING FOUNDATION / ELIGIBLE EXTENSION"; kind = "columns"; leftTitle = "OCT"; left = "Correctness-oriented scientific language`nTests / units / packages / artifacts"; rightTitle = "SDSL-V + PROMETHEUS"; right = "Typed GPU language -> HLSL -> SPIR-V`nVulkan execution runtime"; note = "The foundations predate Build Week." },
    [ordered]@{ start = 18; title = "Exact repository boundary"; kicker = "23 ELIGIBLE FUNCTIONAL COMMITS"; kind = "terminal"; body = $timeline; note = "No July 13-14 work is inferred from framing." },
    [ordered]@{ start = 31; title = "ROALoop"; kicker = "PERSISTENT REVIEW / EPHEMERAL AUTHORS / HUMAN AUTHORITY"; kind = "flow"; body = "PERSISTENT CHATGPT REVIEWER`nretains product context + reviews evidence + writes bounded prompt`n`n        -> FRESH GPT-5.6 CODEX AUTHOR TASK ->`n`nre-ground in repository + implement vertical + test + artifact + handoff`n`n        -> HUMAN STOP / CONTINUE DECISION ->"; note = "Repository tests, reports, and artifacts are the durable memory." },
    [ordered]@{ start = 42; title = "Production-owned fused reduction"; kicker = "adc527d / GPT-5.6 SOL"; kind = "split"; leftTitle = "softmax_fused.sdslv"; left = (Get-Lines "internal/prometheus/shaders/sdslv/production/reduction/softmax_fused.sdslv" 0 34); rightTitle = "M39b evidence contract"; right = (Get-Lines "internal/prometheus/DevelopmentReport/PROMETHEUS_M39B_FUSED_REDUCTION_REACTOR.md" 0 34); note = "Typed source, generated SPIR-V, Vulkan lifecycle, oracle, tests, benchmarks." },
    [ordered]@{ start = 53; title = "SDSL-V gained canonical graphics compilation"; kicker = "f2d4ea8 / GPT-5.6 SOL"; kind = "split"; leftTitle = "CanonicalGraphicsProgram.sdslvvalid"; left = (Get-Lines "Examples/SDSL-V/conformance/graphics/CanonicalGraphicsProgram.sdslvvalid" 0 38); rightTitle = "ForwardTextured.vertex.hlsl"; right = (Get-Lines "Examples/SDSL-V/conformance/artifacts/ForwardTextured.vertex.hlsl" 0 38); note = "Vertex + pixel compiler/toolchain capability. Not a graphics engine." },
    [ordered]@{ start = 63; title = "A bounded transformer vertical"; kicker = "SUCCESSIVE EPHEMERAL CODEX AUTHOR TASKS"; kind = "terminal"; body = $ladder; note = "M47 is a complete bounded block; M48 is a fixed four-block stack." },
    [ordered]@{ start = 78; title = "Measured hardware evidence - including failure"; kicker = "WINDOWS / NVIDIA RTX 3070 / VULKAN VALIDATION"; kind = "terminal"; body = $hardware; note = "Finite output was not accepted as numerical closure." },
    [ordered]@{ start = 94; title = "The real Oct test lane"; kicker = "COMPILED CORRECTNESS CONTRACTS"; kind = "terminal"; body = "> oct test JudgeDemo --execution auto --json`n`n$testOutput"; note = "2 passed / compiled 2 / interpreted fallback 0" },
    [ordered]@{ start = 107; title = "The separate artifact lane"; kicker = "REPRODUCIBLE OUTPUT PROVENANCE"; kind = "terminal"; body = "> oct artifact JudgeDemo --execution interpreted --json`n`n$artifactOutput"; note = "Path + MIME type + 62 bytes + SHA-256" },
    [ordered]@{ start = 120; title = "Codex dogfooded Oct"; kicker = "THE AGENT CHANGED ITS OWN INTERFACE"; kind = "columns"; leftTitle = "BEFORE"; left = "Compiler-shaped MCP surface`noct_check`noct_build`noct_explain_diagnostic`nOverlapping speculative skills"; rightTitle = "AFTER"; right = "Canonical oct test + oct artifact`noct.cli.result.v1`nFallback counts + artifact hashes`nFive bounded hosted tools`nTwo focused repository skills"; note = "Observed local behavior, not guessed agent ergonomics." },
    [ordered]@{ start = 131; title = "Codex-native plugin workflow"; kicker = "GPT-5.6 TERRA PRODUCTIZATION + DOGFOOD"; kind = "flow"; body = "INSPECT FIXTURE`n        -> oct test --json`n        -> normalized diagnostic`n        -> repair source`n        -> rerun compiled facts`n        -> oct artifact --json`n        -> report exact hash"; note = "Local work delegates semantics to the repository CLI." },
    [ordered]@{ start = 143; title = "Diagnostic -> repair -> passing evidence"; kicker = "DETERMINISTIC RECORDING FIXTURE"; kind = "terminal"; body = $repairOutput; note = "The failure is intentional; the repaired run uses the same canonical command." },
    [ordered]@{ start = 150; title = "PERSISTENT REVIEW"; kicker = "OCT + SDSL-V + PROMETHEUS"; kind = "closing"; body = "EPHEMERAL AUTHORS`nDURABLE EVIDENCE"; note = "github.com/yuechen-li-dev/oct" }
)

$slidesJson = $slides | ConvertTo-Json -Depth 8 -Compress
$template = @'
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Oct Build Week Video</title>
<style>
:root { --bg:#07111f; --panel:#0d1b2d; --panel2:#10243a; --ink:#f4f7fb; --muted:#9fb0c7; --cyan:#67e8f9; --green:#6ee7b7; --amber:#fbbf24; --red:#fb7185; }
* { box-sizing:border-box; }
html,body { width:100%; height:100%; margin:0; overflow:hidden; background:var(--bg); color:var(--ink); font-family:Inter,"Segoe UI",sans-serif; }
body:before { content:""; position:fixed; inset:0; background:radial-gradient(circle at 82% 12%,#123554 0,transparent 36%),linear-gradient(135deg,#07111f 0%,#081625 55%,#07111f 100%); }
#stage { position:relative; width:100vw; height:100vh; }
.slide { position:absolute; inset:0; padding:70px 88px 76px; opacity:0; transform:translateY(12px); transition:opacity .35s ease,transform .35s ease; pointer-events:none; }
.slide.active { opacity:1; transform:none; }
.kicker { color:var(--cyan); font-size:22px; font-weight:750; letter-spacing:.16em; text-transform:uppercase; margin-bottom:16px; }
h1 { font-size:58px; line-height:1.05; margin:0 0 30px; letter-spacing:-.025em; }
.hero { display:grid; place-content:center; text-align:center; padding:110px; }
.hero h1 { color:var(--cyan); font-size:112px; letter-spacing:.08em; margin-bottom:30px; }
.hero .body { font-size:52px; font-weight:750; line-height:1.16; max-width:1450px; }
.closing { display:grid; place-content:center; text-align:center; }
.closing h1 { color:var(--cyan); font-size:76px; }
.closing .body { font-size:64px; font-weight:760; line-height:1.35; }
.terminal,.code { background:#050b13; border:1px solid #203b56; border-radius:16px; padding:26px 30px; font:22px/1.42 "Cascadia Code",Consolas,monospace; white-space:pre-wrap; overflow:hidden; box-shadow:0 18px 60px #0005; }
.terminal { color:#c9f8e7; }
.split,.columns { display:grid; grid-template-columns:1fr 1fr; gap:26px; height:720px; }
.panel { background:linear-gradient(180deg,var(--panel2),var(--panel)); border:1px solid #24435f; border-radius:18px; padding:22px 26px; overflow:hidden; }
.panel-title { color:var(--cyan); font-size:21px; font-weight:750; margin-bottom:16px; letter-spacing:.08em; text-transform:uppercase; }
.panel pre { margin:0; font:19px/1.42 "Cascadia Code",Consolas,monospace; white-space:pre-wrap; color:#dbeafe; }
.columns .panel pre { font:30px/1.55 "Segoe UI",sans-serif; color:var(--ink); }
.flow { background:linear-gradient(180deg,#0c2136,#0a1828); border:1px solid #2b5778; border-radius:22px; padding:38px; color:#e6fbff; white-space:pre-wrap; text-align:center; font:31px/1.48 "Cascadia Code",Consolas,monospace; }
.note { position:absolute; left:88px; right:88px; bottom:30px; color:var(--muted); font-size:21px; display:flex; justify-content:space-between; }
.note:after { content:"OPENAI BUILD WEEK 2026"; color:#64809d; letter-spacing:.09em; }
#progress { position:fixed; left:0; bottom:0; height:7px; background:linear-gradient(90deg,var(--cyan),var(--green)); width:0; z-index:10; }
#clock { position:fixed; right:34px; top:28px; color:#71869f; font:18px "Cascadia Code",monospace; z-index:10; }
</style>
</head>
<body>
<div id="stage"></div><div id="clock"></div><div id="progress"></div>
<script>
const slides=__SLIDES_JSON__;
const total=164;
const stage=document.getElementById('stage');
const esc=s=>String(s??'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));
function slideHtml(s,i){
 const base=`<div class="kicker">${esc(s.kicker)}</div><h1>${esc(s.title)}</h1>`;
 let content='';
 if(s.kind==='hero'||s.kind==='closing') content=`<div class="body">${esc(s.body).replace(/\n/g,'<br>')}</div>`;
 else if(s.kind==='terminal') content=`<div class="terminal">${esc(s.body)}</div>`;
 else if(s.kind==='flow') content=`<div class="flow">${esc(s.body)}</div>`;
 else if(s.kind==='split'||s.kind==='columns') content=`<div class="${s.kind}"><div class="panel"><div class="panel-title">${esc(s.leftTitle)}</div><pre>${esc(s.left)}</pre></div><div class="panel"><div class="panel-title">${esc(s.rightTitle)}</div><pre>${esc(s.right)}</pre></div></div>`;
 return `<section class="slide ${s.kind}" data-index="${i}">${base}${content}<div class="note"><span>${esc(s.note)}</span></div></section>`;
}
stage.innerHTML=slides.map(slideHtml).join('');
const elements=[...document.querySelectorAll('.slide')];
let started=performance.now();
function tick(now){
 const seconds=Math.max(0,(now-started)/1000);
 let index=0; for(let i=0;i<slides.length;i++) if(seconds>=slides[i].start) index=i;
 elements.forEach((el,i)=>el.classList.toggle('active',i===index));
 document.getElementById('progress').style.width=`${Math.min(100,seconds/total*100)}%`;
 const mm=String(Math.floor(seconds/60)).padStart(2,'0'), ss=String(Math.floor(seconds%60)).padStart(2,'0');
 document.getElementById('clock').textContent=`${mm}:${ss} / 02:44`;
 requestAnimationFrame(tick);
}
requestAnimationFrame(tick);
</script>
</body>
</html>
'@
$html = $template.Replace("__SLIDES_JSON__", $slidesJson)
[System.IO.File]::WriteAllText($absoluteOutput, $html, [System.Text.UTF8Encoding]::new($false))
Write-Host "Video presentation: $absoluteOutput"
Write-Host "Slides: $($slides.Count)"
Write-Host "Timeline: 164 seconds"

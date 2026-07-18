# Submission packet validation

Validated locally on 2026-07-17 from the audited eligible head plus this
submission packet. A public CI artifact can exist only after the packet commit
is pushed and a workflow run succeeds.

## Passed checks

| Check | Result |
| --- | --- |
| Commit ledger | 23 unique SHAs exist; author dates exactly match Git and are after the official boundary. |
| Evidence paths and links | Every local Markdown link and every concrete ledger file/test/artifact path resolves. |
| Ledger JSON | `BUILD_WEEK_COMMITS.json` parses as `oct.build-week.commits.v1`. |
| Subtitle file | 13 monotonic, non-overlapping ranges; exact narration token match; ends at 00:02:44. |
| Narration rate | 400 words in 164 seconds, approximately 146.3 WPM. |
| Devpost copy | Project name 61 characters, tagline 172, short description 460, long description 1,885; all within the packet's conservative caps. |
| Judge fixture test | 2 passed; 2 compiled; 0 interpreted fallbacks. |
| Judge fixture artifact | 1 passed; JSON; 62 bytes; SHA-256 `4efc9d55ed0eac0e8401f92f1d6b320e7e4c4b7e09778f1c5b7f9917c484209c`. |
| Diagnostic/repair clip | Intentional String/Int diagnostic returned in structured JSON; prepared repair compiled and passed. |
| Focused Go tests | `cmd/oct-mcp`, `internal/cli`, all `internal/sdslv` packages, and the dogfooding generator passed. |
| Full Go tests | `go test -count=1 -parallel 8 ./...` passed. |
| Current binaries | Current `oct` and `oct-mcp` built locally; `oct --version` and `oct-mcp --help` succeeded. |
| Tracked no-rebuild server | The committed Linux x86-64 `oct-mcp` started under WSL2 and returned `ok` from `/healthz`; its size and SHA-256 match the quickstart. |
| README install | `go install github.com/yuechen-li-dev/oct/cmd/oct@v0.1.0` succeeded with repo-local Go caches and the installed binary ran. |
| Workflow syntax | actionlint v1.7.12 passed the general CI and narrow Build Week judge-artifact workflows. |
| Plugin metadata | plugin and marketplace JSON parse; category is Developer Tools; MCP command is `oct-mcp --stdio`. |
| Public state | GitHub reports a public `main` repository with GPL-3.0; no GitHub Release is claimed. |
| Privacy/placeholders | No private absolute path appears in packet content; only the owner-controlled video URL placeholder remains. The `/feedback` ID is confirmed. |
| Whitespace | `git diff --check` passes after removal of the README Markdown trailing spaces. |

Run the reproducible packet checks with:

```powershell
powershell -NoProfile -File docs/build-week/validate-packet.ps1
powershell -NoProfile -File docs/build-week/recording/verify-recording-assets.ps1
```

## CI repair and artifact status

The public workflow at audited head was failing before jobs started because a
job-level environment expression referenced `runner.temp`, a context unavailable
there. The repaired general workflow now parses and starts, exposing older
cross-platform failures: case-sensitive SDSL-V example paths and missing DXC on
Linux, plus native-manifest/golden hash drift and missing DXC on Windows. Those
failures remain visible rather than being skipped.

The separate `Build Week Judge Artifacts` workflow is intentionally narrow. It
builds `oct` and `oct-mcp` with CGO disabled, runs the deterministic judge test
and artifact fixture on Windows and Linux, and only then uploads the binaries
and plugin. This provides the no-rebuild test bundle without misrepresenting the
broader repository CI as green.

## Deliberate non-reruns

No new live Vulkan run was performed merely to prepare submission copy. The
specialized claims point to committed milestone tests, reports, manifests, and
RTX 3070 JSON. Reproducing them requires the documented Windows Vulkan/DXC/C++
environment; Linux-live and AMD evidence remain absent and unclaimed.

## Documentation gap surfaced

`Language/reference/tooling/35-cli.md` specifies `.octbin` as the user-facing
`oct build` artifact, while `docs/COMPILED_SUPPORT.md` describes the current
Windows `.oct.exe` behavior and `.octbin` as future naming. Because the language
reference is authoritative, this is an existing documentation/implementation
gap. The judge path uses `oct test` and `oct artifact` and does not depend on a
compiled artifact suffix.

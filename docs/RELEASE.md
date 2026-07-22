# Oct 1.0 release checklist

The RC artifact contract is documented in `docs/releases/INSTALL_1_0.md`.
Build Windows with `tools/New-OctRelease.ps1 1.0.0-rc.1 <output-dir>` and Linux
with `tools/new_oct_release.sh 1.0.0-rc.1 <output-dir>`. Each produces a
versioned archive and `checksums.sha256`; verify with `Get-FileHash` on Windows
or `sha256sum -c checksums.sha256` on Linux before extraction.

Do not tag or publish an RC from this checklist. GA changes the injected version
to `1.0.0`, repeats extraction-context verification on Windows and Linux, then
requires explicit human approval before `git tag v1.0.0` and publication.

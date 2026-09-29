# Oct release process

The current release is 1.1.0. Read the [release notes](releases/OCT_1_1_RELEASE_NOTES.md) and [installation guide](releases/INSTALL_1_1.md) before packaging. The [1.0 release plan](releases/OCT_1_0_RELEASE_PLAN.md) is historical evidence for that release.

## Build and verify

Start from a clean, reviewed `main` revision with green GitHub CI. Build both supported archives with the repository scripts:

```powershell
.\tools\New-OctRelease.ps1 1.1.0 .tmp\release-1.1.0
```

```sh
./tools/new_oct_release.sh 1.1.0 .tmp/release-1.1.0
```

The output must contain `oct-1.1.0-windows-amd64.zip`, `oct-1.1.0-linux-amd64.tar.gz`, and `checksums.sha256`. Verify both hashes independently; inspect archive paths and required files, extract each into a fresh directory outside the checkout, then run `oct version`, `help`, interpreted `run`, native `build` and execution, compiled `test` with zero fallback, and `fmt`. Run the compiled Octagon load fixture (`Language/Data/Octagon/Load/valid/load_laser_experiment.octest`) through each extracted CLI as part of this check: its generated program needs the packaged `runtime/internal/dimension` source. Confirm the archive contains the correct `INSTALL.md`, compiler runtime, and sidecars. Keep generated archives and temporary output out of Git.

Use the green CI matrix and release-shaped smoke tests as evidence. Do not publish if any hash, version, archive entry, native execution, or required test disagrees with the intended release.

## Publish

Create an annotated `v1.1.0` tag at the verified revision and push it. Create a GitHub release using `OCT_1_1_RELEASE_NOTES.md` as the body, then upload only the two verified archives and `checksums.sha256`. Check the published asset names, sizes, and hashes against the local manifest and ensure the README installation command resolves to the published tag.

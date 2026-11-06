# Prometheus contracts

`valid/` runs without a GPU. It covers ordinary blocks, automatic FFT precision,
source immutability, and recoverable input validation. Run both execution lanes:

```powershell
go run ./cmd/oct test Language/Prometheus/valid --execution compiled
go run ./cmd/oct test Language/Prometheus/valid --execution interpreted
```

`gpu/` requires the current native reactor, its shader package, Vulkan, and
`shaderFloat64`. These tests must pass through real GPU execution. Run the same
two commands with `gpu` in place of `valid` on the qualified native host.

`unavailable/` requires an explicitly missing reactor. It verifies recoverable
FFT failure and terminal matrix failure instead of CPU substitution:

```powershell
$env:OCT_PROMETHEUS_REACTOR = 'C:/nonexistent/m12-reactor.dll'
go run ./cmd/oct test Language/Prometheus/unavailable --execution auto
Remove-Item Env:OCT_PROMETHEUS_REACTOR
```

These are separate environment lanes; running the parent directory as one
environment would ask for conflicting native setup. Go integration tests only
orchestrate these contracts. They do not redefine their numerical semantics.

`bench/` keeps a CPU matrix benchmark beside a strict GPU benchmark. With a
missing reactor, `oct bench Language/Prometheus/bench` must report CPU success
and GPU failure. Successful native GPU runs preserve backend/environment
metadata in the benchmark's Octagon report.

# Kaiju source provenance

The production sidecar uses the Kaiju raw Vulkan binding from the explicit
local source slot `out/kaiju-audit/src`, which must be a checkout of
`https://github.com/KaijuEngine/kaiju` at
`ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2`.

This is an interim local-fork ownership model: `tools/build_sidecars` must
verify the checkout commit before it builds the nested sidecar module. Kaiju is
an implementation dependency of this executable only; no package in the Oct
root module imports it. No Kaiju prebuilt libraries, game systems, windowing,
audio, physics, editor code, or `src/libs` assets are distributed by this
sidecar.

Upstream is MIT licensed. The final packaging step must copy the exact upstream
MIT notice into this directory and record any fork patch commit.

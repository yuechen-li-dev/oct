# Kaiju Vulkan sidecar spike

This command is an isolated evaluation artifact. It is not part of Oct's normal
module graph, sidecar build list, or production runtime.

The source authority is Kaiju commit
`ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2`. Prepare the ignored checkout from
the Oct repository root:

```powershell
git clone https://github.com/KaijuEngine/kaiju.git out/kaiju-audit
git -C out/kaiju-audit checkout ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2
```

Build and run on Windows with Go 1.25 or newer, CGO, a 64-bit GCC-compatible C
compiler, Vulkan headers, and a Vulkan loader/runtime:

```powershell
go build -C tools/octxiliary_kaiju_vulkan_spike -o ../../out/kaiju-spike/octxiliary-kaiju-vulkan.exe .
out/kaiju-spike/octxiliary-kaiju-vulkan.exe --request request.json
```

The proof accepts one JSON request file and writes exactly one JSON response to
stdout. It supports set 0 storage buffers, an explicit entry point, push
constants, dispatch geometry, warmups, GPU timestamp samples, and selected
buffer readback. Oct remains responsible for the resource ABI and result
interpretation.

Delete `tools/octxiliary_kaiju_vulkan_spike` and the ignored
`out/kaiju-audit` / `out/kaiju-spike` directories to roll the spike back.

module github.com/yuechen-li-dev/oct/tools/octxiliary_kaiju_vulkan_spike

go 1.25.0

require kaijuengine.com v0.0.0

// Audit-only source checkout. See PINNED_KAIJU_COMMIT and README.md.
replace kaijuengine.com => ../../out/kaiju-audit/src

module github.com/yuechen-li-dev/oct-sidecars/kaiju-vulkan

go 1.25.0

require (
    github.com/yuechen-li-dev/oct v0.0.0
    kaijuengine.com v0.0.0
)

// Deliberate local pinned-source ownership. build_sidecars verifies the commit
// before invoking this module; production Oct packages never import Kaiju.
replace github.com/yuechen-li-dev/oct => ../..
replace kaijuengine.com => ../../out/kaiju-audit/src

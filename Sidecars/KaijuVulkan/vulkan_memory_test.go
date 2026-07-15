package main

import (
	"testing"

	vk "kaijuengine.com/rendering/vulkan"
	vc "kaijuengine.com/rendering/vulkan_const"
)

func TestSelectBufferMemoryTypePrefersHostVisibleDeviceLocal(t *testing.T) {
	host := vk.MemoryPropertyFlags(vc.MemoryPropertyHostVisibleBit | vc.MemoryPropertyHostCoherentBit)
	localHost := host | vk.MemoryPropertyFlags(vc.MemoryPropertyDeviceLocalBit)
	available := []vk.MemoryPropertyFlags{host, vk.MemoryPropertyFlags(vc.MemoryPropertyDeviceLocalBit), localHost}

	if got, ok := selectBufferMemoryType(0b111, available, ""); !ok || got != 2 {
		t.Fatalf("default selection = (%d, %v), want combined memory type 2", got, ok)
	}
	if got, ok := selectBufferMemoryType(0b111, available, auditMemoryHostVisible); !ok || got != 0 {
		t.Fatalf("host-visible audit selection = (%d, %v), want memory type 0", got, ok)
	}
	if got, ok := selectBufferMemoryType(0b011, available, auditMemoryHostDeviceLocal); ok {
		t.Fatalf("forced combined selection unexpectedly succeeded with type %d", got)
	}
	if got, ok := selectBufferMemoryType(0b011, available, ""); !ok || got != 0 {
		t.Fatalf("fallback selection = (%d, %v), want host-visible memory type 0", got, ok)
	}
}

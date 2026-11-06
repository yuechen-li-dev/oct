package prometheus

import "testing"

func TestStrictGpuAdmissionRejectsSoftwareAndUnavailableBackends(t *testing.T) {
	for _, caps := range []reactorCaps{{Available: false, BackendType: 2}, {Available: true, BackendType: 3}, {Available: true, BackendType: 0}} {
		runtime := &nativeRuntime{caps: caps}
		if err := runtime.requireHardwareGPU(); err == nil {
			t.Fatalf("non-hardware backend admitted: %+v", caps)
		}
	}
	if err := (&nativeRuntime{caps: reactorCaps{Available: true, BackendType: 2}}).requireHardwareGPU(); err != nil {
		t.Fatal(err)
	}
}

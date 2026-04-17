package prometheus

/*
#cgo CXXFLAGS: -std=c++17
#cgo LDFLAGS: -lstdc++ -static-libgcc -static-libstdc++
#include "native/bridge.h"
*/
import "C"

import (
	"fmt"
	"unsafe"
)

type nativeRuntime struct {
	handle *C.prometheus_runtime
}

func newNativeRuntime() (*nativeRuntime, error) {
	var handle *C.prometheus_runtime
	rc := int(C.prometheus_runtime_create(&handle))
	if rc != 0 {
		if rc == int(C.PROMETHEUS_NATIVE_ERR_UNAVAILABLE) {
			return nil, errNativeUnavailable
		}
		return nil, fmt.Errorf("prometheus runtime init failed code=%d", rc)
	}
	if handle == nil {
		return nil, fmt.Errorf("prometheus runtime init returned nil handle")
	}
	return &nativeRuntime{handle: handle}, nil
}

func (r *nativeRuntime) Close() {
	if r == nil || r.handle == nil {
		return
	}
	C.prometheus_runtime_destroy(r.handle)
	r.handle = nil
}

func (r *nativeRuntime) SGEMM(m, n, k int, a, b []float32) ([]float32, RunStatus, error) {
	if len(a) != m*k || len(b) != k*n {
		return nil, ErrorStatus(StageInit, int(C.PROMETHEUS_NATIVE_ERR_INVALID_ARGUMENT)), fmt.Errorf("invalid matrix lengths")
	}
	c := make([]float32, m*n)
	stage := C.int(C.PROMETHEUS_NATIVE_STAGE_UNKNOWN)
	code := C.int(C.PROMETHEUS_NATIVE_OK)
	rc := int(C.prometheus_sgemm(
		r.handle,
		C.int(m),
		C.int(n),
		C.int(k),
		(*C.float)(unsafe.Pointer(&a[0])),
		(*C.float)(unsafe.Pointer(&b[0])),
		(*C.float)(unsafe.Pointer(&c[0])),
		&stage,
		&code,
	))
	if rc != 0 {
		status := ErrorStatus(mapNativeStage(int(stage)), int(code))
		return nil, status, fmt.Errorf("prometheus execution failed stage=%s code=%d", status.ErrorStage, status.ErrorCode)
	}
	return c, OkStatus(), nil
}

func mapNativeStage(stage int) ErrorStage {
	switch stage {
	case int(C.PROMETHEUS_NATIVE_STAGE_INIT):
		return StageInit
	case int(C.PROMETHEUS_NATIVE_STAGE_TRANSFER_IN):
		return StageTransferIn
	case int(C.PROMETHEUS_NATIVE_STAGE_SUBMIT):
		return StageSubmit
	case int(C.PROMETHEUS_NATIVE_STAGE_TRANSFER_OUT):
		return StageTransferOut
	case int(C.PROMETHEUS_NATIVE_STAGE_CLEANUP):
		return StageCleanup
	default:
		return StageUnknown
	}
}
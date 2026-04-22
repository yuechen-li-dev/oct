//go:build windows && cgo

package prometheus

/*
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdlib.h>

typedef uint32_t (__cdecl *oct_prom_abi_fn)(void);
typedef int (__cdecl *oct_prom_create_fn)(void*, void**);
typedef int (__cdecl *oct_prom_destroy_fn)(void*);
typedef int (__cdecl *oct_prom_probe_fn)(void*, void*);
typedef int (__cdecl *oct_prom_sgemm_fn)(void*, const float*, const float*, float*, uint32_t, uint32_t, uint32_t, uint32_t*, int*);
typedef int (__cdecl *oct_prom_submit_async_fn)(void*, const float*, const float*, uint32_t, uint32_t, uint32_t, int*, uint32_t*, int*);
typedef int (__cdecl *oct_prom_query_async_fn)(void*, int, void*);
typedef int (__cdecl *oct_prom_consume_async_fn)(void*, int, float*, uint32_t, uint32_t*, int*);
typedef int (__cdecl *oct_prom_abandon_async_fn)(void*, int);

typedef struct oct_prom_caps {
	uint32_t available;
	uint32_t backend_type;
	uint32_t reason_code;
} oct_prom_caps;

typedef struct oct_prom_cfg {
	uint32_t struct_size;
	uint32_t test_flags;
} oct_prom_cfg;

typedef struct oct_prom_async_status {
	uint32_t lifecycle_state;
	uint32_t stage;
	int detail_code;
	uint32_t ready;
	uint32_t failed;
	uint32_t consumed;
	uint32_t outstanding_tasks;
} oct_prom_async_status;

static HMODULE oct_prom_load_library(const char* path) {
	return LoadLibraryA(path);
}

static FARPROC oct_prom_get_proc(HMODULE handle, const char* symbol) {
	return GetProcAddress(handle, symbol);
}

static DWORD oct_prom_last_error(void) {
	return GetLastError();
}

static int oct_prom_free_library(HMODULE handle) {
	return FreeLibrary(handle) ? 0 : 1;
}

static uint32_t oct_prom_call_abi(FARPROC symbol) {
	return ((oct_prom_abi_fn)symbol)();
}

static int oct_prom_call_create(FARPROC symbol, oct_prom_cfg* cfg, uintptr_t* out_handle) {
	void* handle = NULL;
	int status = ((oct_prom_create_fn)symbol)(cfg, &handle);
	if (out_handle != NULL) {
		*out_handle = (uintptr_t)handle;
	}
	return status;
}

static int oct_prom_call_destroy(FARPROC symbol, uintptr_t handle) {
	return ((oct_prom_destroy_fn)symbol)((void*)handle);
}

static int oct_prom_call_probe(FARPROC symbol, uintptr_t handle, oct_prom_caps* out_caps) {
	return ((oct_prom_probe_fn)symbol)((void*)handle, out_caps);
}

static int oct_prom_call_sgemm(FARPROC symbol,
	uintptr_t handle,
	const float* a,
	const float* b,
	float* c,
	uint32_t m,
	uint32_t n,
	uint32_t k,
	uint32_t* out_stage,
	int* out_detail_code) {
	return ((oct_prom_sgemm_fn)symbol)((void*)handle, a, b, c, m, n, k, out_stage, out_detail_code);
}

static int oct_prom_call_submit_async(FARPROC symbol,
	uintptr_t handle,
	const float* a,
	const float* b,
	uint32_t m,
	uint32_t n,
	uint32_t k,
	int* out_task_id,
	uint32_t* out_stage,
	int* out_detail_code) {
	return ((oct_prom_submit_async_fn)symbol)((void*)handle, a, b, m, n, k, out_task_id, out_stage, out_detail_code);
}

static int oct_prom_call_query_async(FARPROC symbol, uintptr_t handle, int task_id, oct_prom_async_status* out_status) {
	return ((oct_prom_query_async_fn)symbol)((void*)handle, task_id, out_status);
}

static int oct_prom_call_consume_async(FARPROC symbol,
	uintptr_t handle,
	int task_id,
	float* c,
	uint32_t c_len,
	uint32_t* out_stage,
	int* out_detail_code) {
	return ((oct_prom_consume_async_fn)symbol)((void*)handle, task_id, c, c_len, out_stage, out_detail_code);
}

static int oct_prom_call_abandon_async(FARPROC symbol, uintptr_t handle, int task_id) {
	return ((oct_prom_abandon_async_fn)symbol)((void*)handle, task_id);
}
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

type winDLLLoader struct{}

type winDLLLibrary struct {
	handle C.HMODULE
}

func defaultReactorLoader() reactorLoader {
	return winDLLLoader{}
}

func (winDLLLoader) Open(path string) (dynamicLibrary, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.oct_prom_load_library(cPath)
	if handle == nil {
		return nil, fmt.Errorf("LoadLibrary %s failed: win32=%d", path, uint32(C.oct_prom_last_error()))
	}
	return &winDLLLibrary{handle: handle}, nil
}

func (l *winDLLLibrary) Resolve(symbol string) (any, error) {
	if l == nil || l.handle == nil {
		return nil, fmt.Errorf("reactor library handle unavailable")
	}
	cSymbol := C.CString(symbol)
	defer C.free(unsafe.Pointer(cSymbol))

	sym := C.oct_prom_get_proc(l.handle, cSymbol)
	if sym == nil {
		return nil, fmt.Errorf("GetProcAddress %s failed: win32=%d", symbol, uint32(C.oct_prom_last_error()))
	}

	switch symbol {
	case reactorSymbolABIVersion:
		return reactorABI(func() uint32 {
			return uint32(C.oct_prom_call_abi(sym))
		}), nil
	case reactorSymbolCreate:
		return reactorCreate(func(cfg reactorCreateConfig) (reactorRuntimeHandle, error) {
			var raw C.uintptr_t
			cCfg := C.oct_prom_cfg{
				struct_size: C.uint32_t(C.sizeof_oct_prom_cfg),
				test_flags:  C.uint32_t(cfg.TestFlags),
			}
			status := int(C.oct_prom_call_create(sym, &cCfg, &raw))
			if status != 0 {
				return reactorRuntimeHandle{}, fmt.Errorf("reactor create status=%d", status)
			}
			return reactorRuntimeHandle{ptr: uintptr(raw)}, nil
		}), nil
	case reactorSymbolDestroy:
		return reactorDestroy(func(handle reactorRuntimeHandle) {
			_ = C.oct_prom_call_destroy(sym, C.uintptr_t(handle.ptr))
		}), nil
	case reactorSymbolProbe:
		return reactorProbe(func(handle reactorRuntimeHandle) (reactorCaps, error) {
			var caps C.oct_prom_caps
			status := int(C.oct_prom_call_probe(sym, C.uintptr_t(handle.ptr), &caps))
			if status != 0 {
				return reactorCaps{}, fmt.Errorf("reactor probe status=%d", status)
			}
			return reactorCaps{
				Available:   caps.available != 0,
				BackendType: uint32(caps.backend_type),
				ReasonCode:  uint32(caps.reason_code),
			}, nil
		}), nil
	case reactorSymbolSGEMM:
		return reactorSGEMM(func(handle reactorRuntimeHandle, m, n, k int, a, b []float32) ([]float32, reactorCallStatus, error) {
			out := make([]float32, m*n)
			var stage C.uint32_t
			var detail C.int
			status := int(C.oct_prom_call_sgemm(
				sym,
				C.uintptr_t(handle.ptr),
				floatSlicePointer(a),
				floatSlicePointer(b),
				floatSlicePointer(out),
				C.uint32_t(m),
				C.uint32_t(n),
				C.uint32_t(k),
				&stage,
				&detail,
			))
			callStatus := reactorCallStatus{StageCode: uint32(stage), DetailCode: int(detail)}
			if status != 0 {
				return nil, callStatus, fmt.Errorf("reactor sgemm status=%d", status)
			}
			return out, callStatus, nil
		}), nil
	case reactorSymbolSubmitAsync:
		return reactorSubmitAsync(func(handle reactorRuntimeHandle, m, n, k int, a, b []float32) (int, reactorCallStatus, error) {
			var taskID C.int
			var stage C.uint32_t
			var detail C.int
			status := int(C.oct_prom_call_submit_async(
				sym,
				C.uintptr_t(handle.ptr),
				floatSlicePointer(a),
				floatSlicePointer(b),
				C.uint32_t(m),
				C.uint32_t(n),
				C.uint32_t(k),
				&taskID,
				&stage,
				&detail,
			))
			callStatus := reactorCallStatus{StageCode: uint32(stage), DetailCode: int(detail)}
			if status != 0 {
				return 0, callStatus, fmt.Errorf("reactor async submit status=%d", status)
			}
			return int(taskID), callStatus, nil
		}), nil
	case reactorSymbolQueryAsync:
		return reactorQueryAsync(func(handle reactorRuntimeHandle, taskID int) (reactorAsyncStatus, error) {
			var statusOut C.oct_prom_async_status
			status := int(C.oct_prom_call_query_async(sym, C.uintptr_t(handle.ptr), C.int(taskID), &statusOut))
			if status != 0 {
				return reactorAsyncStatus{}, fmt.Errorf("reactor async query status=%d", status)
			}
			return reactorAsyncStatus{
				LifecycleState:   uint32(statusOut.lifecycle_state),
				StageCode:        uint32(statusOut.stage),
				DetailCode:       int(statusOut.detail_code),
				Ready:            statusOut.ready != 0,
				Failed:           statusOut.failed != 0,
				Consumed:         statusOut.consumed != 0,
				OutstandingTasks: uint32(statusOut.outstanding_tasks),
			}, nil
		}), nil
	case reactorSymbolConsumeAsync:
		return reactorConsumeAsync(func(handle reactorRuntimeHandle, taskID int, out []float32) (reactorCallStatus, error) {
			var stage C.uint32_t
			var detail C.int
			status := int(C.oct_prom_call_consume_async(
				sym,
				C.uintptr_t(handle.ptr),
				C.int(taskID),
				floatSlicePointer(out),
				C.uint32_t(len(out)),
				&stage,
				&detail,
			))
			callStatus := reactorCallStatus{StageCode: uint32(stage), DetailCode: int(detail)}
			if status != 0 {
				return callStatus, fmt.Errorf("reactor async consume status=%d", status)
			}
			return callStatus, nil
		}), nil
	case reactorSymbolAbandonAsync:
		return reactorAbandonAsync(func(handle reactorRuntimeHandle, taskID int) error {
			status := int(C.oct_prom_call_abandon_async(sym, C.uintptr_t(handle.ptr), C.int(taskID)))
			if status != 0 {
				return fmt.Errorf("reactor async abandon status=%d", status)
			}
			return nil
		}), nil
	default:
		return nil, fmt.Errorf("unsupported reactor symbol %q", symbol)
	}
}

func (l *winDLLLibrary) Close() error {
	if l == nil || l.handle == nil {
		return nil
	}
	if rc := C.oct_prom_free_library(l.handle); rc != 0 {
		return fmt.Errorf("FreeLibrary failed: win32=%d", uint32(C.oct_prom_last_error()))
	}
	l.handle = nil
	return nil
}

func floatSlicePointer(values []float32) *C.float {
	if len(values) == 0 {
		return nil
	}
	return (*C.float)(unsafe.Pointer(&values[0]))
}

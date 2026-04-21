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

typedef struct oct_prom_caps {
	uint32_t available;
	uint32_t backend_type;
	uint32_t reason_code;
} oct_prom_caps;

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

static int oct_prom_call_create(FARPROC symbol, uintptr_t* out_handle) {
	void* handle = NULL;
	int status = ((oct_prom_create_fn)symbol)(NULL, &handle);
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
		return reactorCreate(func() (reactorRuntimeHandle, error) {
			var raw C.uintptr_t
			status := int(C.oct_prom_call_create(sym, &raw))
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

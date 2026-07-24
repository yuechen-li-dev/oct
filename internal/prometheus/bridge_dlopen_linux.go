//go:build linux && cgo

package prometheus

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "native/reactor_api.h"

typedef uint32_t (*oct_prom_abi_fn)(void);
typedef int (*oct_prom_create_fn)(void*, void**);
typedef int (*oct_prom_destroy_fn)(void*);
typedef int (*oct_prom_probe_fn)(void*, void*);
typedef int (*oct_prom_sgemm_fn)(void*, const float*, const float*, float*, uint32_t, uint32_t, uint32_t, uint32_t*, int*);
typedef int (*oct_prom_submit_async_fn)(void*, const float*, const float*, uint32_t, uint32_t, uint32_t, int*, uint32_t*, int*);
typedef int (*oct_prom_query_async_fn)(void*, int, void*);
typedef int (*oct_prom_consume_async_fn)(void*, int, float*, uint32_t, uint32_t*, int*);
typedef int (*oct_prom_abandon_async_fn)(void*, int);
typedef int (*oct_prom_gemma4e2b_input_rmsnorm_fn)(void*, const void*, void*);
typedef int (*oct_prom_gemma4e2b_rope_fn)(void*, const void*, void*);
typedef int (*oct_prom_gemma4e2b_head_rmsnorm_rope_fn)(void*, const void*, void*);
typedef int (*oct_prom_gemma4e2b_attention_scores_fn)(void*, const PrometheusGemma4E2BM1AttentionScoresRequest*, PrometheusGemma4E2BM1AttentionScoresResult*);

typedef struct oct_prom_caps {
	uint32_t available;
	uint32_t backend_type;
	uint32_t reason_code;
} oct_prom_caps;

typedef struct oct_prom_cfg {
	uint32_t struct_size;
	uint32_t test_flags;
	uint32_t p15_shadow_canary_enabled;
	uint32_t async_test_flags;
	uint32_t batch_ring_depth;
	uint32_t reduction_test_flags;
	uint32_t reduction_ring_depth;
	const char* shader_package_root;
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

typedef struct oct_prom_gemma4e2b_input_rmsnorm_request {
	uint32_t struct_size;
	const float* input;
	const float* weight;
	float* output;
	float* inv_rms_output;
	uint64_t input_element_count;
	uint64_t weight_element_count;
	uint64_t output_element_count;
	uint64_t inv_rms_output_element_count;
	uint32_t tokens;
	uint32_t model_width;
	uint32_t input_row_stride;
	float epsilon;
	uint64_t input_generation;
	uint64_t weight_generation;
	uint64_t exact_source_hash;
} oct_prom_gemma4e2b_input_rmsnorm_request;

typedef struct oct_prom_gemma4e2b_input_rmsnorm_result {
	uint32_t struct_size;
	uint32_t stage;
	int32_t detail_code;
	uint32_t output_written;
	uint32_t matched_input;
	uint64_t input_hash;
	uint64_t weight_hash;
	uint64_t output_hash;
	uint64_t inv_rms_hash;
	uint32_t submit_count;
	uint32_t final_readback_count;
	uint32_t no_product_intermediate_readback_change;
	uint32_t reserved0;
	uint64_t retained_bytes;
	uint64_t buffer_allocation_count;
	uint64_t buffer_reuse_count;
	uint64_t descriptor_update_count;
	uint64_t pipeline_create_count;
	uint64_t command_buffer_reuse_count;
	uint64_t reduction_gpu_ns;
	uint64_t final_reduction_gpu_ns;
	uint64_t inv_rms_gpu_ns;
	uint64_t apply_gpu_ns;
	uint64_t end_to_end_ns;
} oct_prom_gemma4e2b_input_rmsnorm_result;

typedef struct oct_prom_gemma4e2b_rope_request {
	uint32_t struct_size;
	const float* source;
	const float* cosine;
	const float* sine;
	float* output;
	uint64_t source_element_count;
	uint64_t cosine_element_count;
	uint64_t sine_element_count;
	uint64_t output_element_count;
	uint32_t tokens;
	uint32_t heads;
	uint32_t head_width;
	uint32_t reserved0;
	uint64_t source_generation;
	uint64_t table_generation;
} oct_prom_gemma4e2b_rope_request;

typedef struct oct_prom_gemma4e2b_rope_result {
	uint32_t struct_size;
	uint32_t stage;
	int32_t detail_code;
	uint32_t output_written;
	uint32_t dispatch_count;
	uint64_t source_hash;
	uint64_t cosine_hash;
	uint64_t sine_hash;
	uint64_t output_hash;
	uint64_t buffer_allocation_count;
	uint64_t buffer_reuse_count;
	uint64_t descriptor_update_count;
	uint64_t pipeline_create_count;
	uint64_t command_buffer_reuse_count;
} oct_prom_gemma4e2b_rope_result;

typedef struct oct_prom_gemma4e2b_head_rmsnorm_rope_request {
	uint32_t struct_size; const float* input; const float* weight; const float* cosine; const float* sine; float* output;
	uint64_t input_element_count; uint64_t weight_element_count; uint64_t cosine_element_count; uint64_t sine_element_count; uint64_t output_element_count;
	uint32_t tokens; uint32_t heads; uint32_t head_width; uint32_t reserved0; float epsilon;
	uint64_t input_generation; uint64_t weight_generation; uint64_t table_generation; uint64_t exact_source_hash;
} oct_prom_gemma4e2b_head_rmsnorm_rope_request;
typedef struct oct_prom_gemma4e2b_head_rmsnorm_rope_result {
	uint32_t struct_size; uint32_t stage; int32_t detail_code; uint32_t output_written; uint32_t dispatch_count;
	uint32_t resident_source_bound; uint32_t normalized_readback_count;
	uint64_t source_byte_offset; uint64_t source_byte_range; uint64_t destination_byte_offset; uint64_t destination_byte_range;
	uint64_t input_hash; uint64_t weight_hash; uint64_t output_hash; uint64_t buffer_allocation_count; uint64_t buffer_reuse_count;
	uint64_t descriptor_update_count; uint64_t pipeline_create_count; uint64_t command_buffer_reuse_count; uint64_t observed_weight_generation; uint64_t requested_weight_generation;
} oct_prom_gemma4e2b_head_rmsnorm_rope_result;

static void* oct_prom_dlopen(const char* path) {
	return dlopen(path, RTLD_NOW | RTLD_LOCAL);
}

static void* oct_prom_dlsym(void* handle, const char* symbol) {
	return dlsym(handle, symbol);
}

static const char* oct_prom_dlerror(void) {
	return dlerror();
}

static int oct_prom_dlclose(void* handle) {
	return dlclose(handle);
}

static uint32_t oct_prom_call_abi(void* symbol) {
	return ((oct_prom_abi_fn)symbol)();
}

static int oct_prom_call_create(void* symbol, oct_prom_cfg* cfg, uintptr_t* out_handle) {
	void* handle = NULL;
	int status = ((oct_prom_create_fn)symbol)(cfg, &handle);
	if (out_handle != NULL) {
		*out_handle = (uintptr_t)handle;
	}
	return status;
}

static int oct_prom_call_destroy(void* symbol, uintptr_t handle) {
	return ((oct_prom_destroy_fn)symbol)((void*)handle);
}

static int oct_prom_call_probe(void* symbol, uintptr_t handle, oct_prom_caps* out_caps) {
	return ((oct_prom_probe_fn)symbol)((void*)handle, out_caps);
}

static int oct_prom_call_sgemm(void* symbol,
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

static int oct_prom_call_submit_async(void* symbol,
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

static int oct_prom_call_query_async(void* symbol, uintptr_t handle, int task_id, oct_prom_async_status* out_status) {
	return ((oct_prom_query_async_fn)symbol)((void*)handle, task_id, out_status);
}

static int oct_prom_call_consume_async(void* symbol,
	uintptr_t handle,
	int task_id,
	float* c,
	uint32_t c_len,
	uint32_t* out_stage,
	int* out_detail_code) {
	return ((oct_prom_consume_async_fn)symbol)((void*)handle, task_id, c, c_len, out_stage, out_detail_code);
}

static int oct_prom_call_abandon_async(void* symbol, uintptr_t handle, int task_id) {
	return ((oct_prom_abandon_async_fn)symbol)((void*)handle, task_id);
}

static int oct_prom_call_gemma4e2b_input_rmsnorm(void* symbol,
	uintptr_t handle,
	const float* input,
	const float* weight,
	float* output,
	float* inv_rms_output,
	uint64_t input_element_count,
	uint64_t weight_element_count,
	uint64_t output_element_count,
	uint64_t inv_rms_output_element_count,
	uint32_t tokens,
	uint32_t model_width,
	uint32_t input_row_stride,
	float epsilon,
	uint64_t input_generation,
	uint64_t weight_generation,
	uint64_t exact_source_hash,
	oct_prom_gemma4e2b_input_rmsnorm_result* out_result) {
	oct_prom_gemma4e2b_input_rmsnorm_request request;
	memset(&request, 0, sizeof(request));
	request.struct_size = (uint32_t)sizeof(request);
	request.input = input;
	request.weight = weight;
	request.output = output;
	request.inv_rms_output = inv_rms_output;
	request.input_element_count = input_element_count;
	request.weight_element_count = weight_element_count;
	request.output_element_count = output_element_count;
	request.inv_rms_output_element_count = inv_rms_output_element_count;
	request.tokens = tokens;
	request.model_width = model_width;
	request.input_row_stride = input_row_stride;
	request.epsilon = epsilon;
	request.input_generation = input_generation;
	request.weight_generation = weight_generation;
	request.exact_source_hash = exact_source_hash;
	return ((oct_prom_gemma4e2b_input_rmsnorm_fn)symbol)((void*)handle, &request, out_result);
}

static int oct_prom_call_gemma4e2b_rope(void* symbol, uintptr_t handle,
	const float* source, const float* cosine, const float* sine, float* output,
	uint64_t source_count, uint64_t cosine_count, uint64_t sine_count, uint64_t output_count,
	uint32_t tokens, uint32_t heads, uint32_t head_width,
	uint64_t source_generation, uint64_t table_generation,
	oct_prom_gemma4e2b_rope_result* out_result) {
	oct_prom_gemma4e2b_rope_request request;
	memset(&request, 0, sizeof(request));
	request.struct_size = (uint32_t)sizeof(request);
	request.source = source; request.cosine = cosine; request.sine = sine; request.output = output;
	request.source_element_count = source_count; request.cosine_element_count = cosine_count;
	request.sine_element_count = sine_count; request.output_element_count = output_count;
	request.tokens = tokens; request.heads = heads; request.head_width = head_width;
	request.source_generation = source_generation; request.table_generation = table_generation;
	return ((oct_prom_gemma4e2b_rope_fn)symbol)((void*)handle, &request, out_result);
}

static int oct_prom_call_gemma4e2b_head_rmsnorm_rope(void* symbol, uintptr_t handle,
	const float* input, const float* weight, const float* cosine, const float* sine, float* output,
	uint64_t input_count, uint64_t weight_count, uint64_t cosine_count, uint64_t sine_count, uint64_t output_count,
	uint32_t tokens, uint32_t heads, uint32_t head_width, float epsilon,
	uint64_t input_generation, uint64_t weight_generation, uint64_t table_generation, uint64_t exact_source_hash,
	oct_prom_gemma4e2b_head_rmsnorm_rope_result* out_result) {
	oct_prom_gemma4e2b_head_rmsnorm_rope_request request; memset(&request, 0, sizeof(request));
	request.struct_size = sizeof(request); request.input = input; request.weight = weight; request.cosine = cosine; request.sine = sine; request.output = output;
	request.input_element_count = input_count; request.weight_element_count = weight_count; request.cosine_element_count = cosine_count; request.sine_element_count = sine_count; request.output_element_count = output_count;
	request.tokens = tokens; request.heads = heads; request.head_width = head_width; request.epsilon = epsilon;
	request.input_generation = input_generation; request.weight_generation = weight_generation; request.table_generation = table_generation; request.exact_source_hash = exact_source_hash;
	return ((oct_prom_gemma4e2b_head_rmsnorm_rope_fn)symbol)((void*)handle, &request, out_result);
}

static int oct_prom_call_gemma4e2b_attention_scores(
	void* symbol, uintptr_t handle,
	const float* query_input, const float* key_input, const float* query_weight, const float* key_weight,
	const float* cosine, const float* sine, float* scores,
	uint64_t query_count, uint64_t key_count, uint64_t query_weight_count, uint64_t key_weight_count,
	uint64_t cosine_count, uint64_t sine_count, uint64_t score_count,
	uint32_t tokens, uint32_t query_heads, uint32_t key_heads, uint32_t head_width, float epsilon, float scale, uint32_t preparation_order,
	uint64_t query_generation, uint64_t key_generation, uint64_t query_weight_generation, uint64_t key_weight_generation,
	uint64_t table_generation, uint64_t query_source_hash, uint64_t key_source_hash,
	PrometheusGemma4E2BM1AttentionScoresResult* out_result) {
	PrometheusGemma4E2BM1AttentionScoresRequest request; memset(&request, 0, sizeof(request));
	request.struct_size = sizeof(request); request.query_input = query_input; request.key_input = key_input;
	request.query_weight = query_weight; request.key_weight = key_weight; request.cosine = cosine; request.sine = sine; request.scores = scores;
	request.query_input_element_count = query_count; request.key_input_element_count = key_count;
	request.query_weight_element_count = query_weight_count; request.key_weight_element_count = key_weight_count;
	request.cosine_element_count = cosine_count; request.sine_element_count = sine_count; request.score_element_count = score_count;
	request.tokens = tokens; request.query_heads = query_heads; request.key_heads = key_heads; request.head_width = head_width; request.epsilon = epsilon; request.scale = scale; request.preparation_order = preparation_order;
	request.query_input_generation = query_generation; request.key_input_generation = key_generation;
	request.query_weight_generation = query_weight_generation; request.key_weight_generation = key_weight_generation;
	request.table_generation = table_generation; request.query_exact_source_hash = query_source_hash; request.key_exact_source_hash = key_source_hash;
	return ((oct_prom_gemma4e2b_attention_scores_fn)symbol)((void*)handle, &request, out_result);
}
*/
import "C"

import (
	"fmt"
	"os"
	"unsafe"
)

type dlLoader struct{}

type dlLibrary struct {
	handle unsafe.Pointer
}

func defaultReactorLoader() reactorLoader {
	return dlLoader{}
}

func (dlLoader) Open(path string) (dynamicLibrary, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))

	handle := C.oct_prom_dlopen(cPath)
	if handle == nil {
		return nil, fmt.Errorf("dlopen %s: %s", path, dlErrorString())
	}
	return &dlLibrary{handle: handle}, nil
}

func (l *dlLibrary) Resolve(symbol string) (any, error) {
	if l == nil || l.handle == nil {
		return nil, fmt.Errorf("reactor library handle unavailable")
	}
	cSymbol := C.CString(symbol)
	defer C.free(unsafe.Pointer(cSymbol))

	sym := C.oct_prom_dlsym(l.handle, cSymbol)
	if sym == nil {
		return nil, fmt.Errorf("dlsym %s: %s", symbol, dlErrorString())
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
				struct_size:               C.uint32_t(C.sizeof_oct_prom_cfg),
				test_flags:                C.uint32_t(cfg.TestFlags),
				p15_shadow_canary_enabled: 0,
				async_test_flags:          0,
				batch_ring_depth:          0,
				reduction_test_flags:      0,
				reduction_ring_depth:      C.uint32_t(cfg.ReductionRingDepth),
			}
			var cRoot *C.char
			if cfg.ShaderPackageRoot != "" {
				cRoot = C.CString(cfg.ShaderPackageRoot)
				defer C.free(unsafe.Pointer(cRoot))
				cCfg.shader_package_root = cRoot
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
	case reactorSymbolGemmaInputRN, reactorSymbolGemmaProjectionActivationRN, reactorSymbolGemmaHeadRN:
		return reactorGemma4E2BM1InputRMSNorm(func(handle reactorRuntimeHandle, input, weight []float32, tokens, modelWidth, inputRowStride uint32, epsilon float32, inputGeneration, weightGeneration, exactSourceHash uint64) ([]float32, []float32, reactorGemma4E2BM1InputRMSNormResult, error) {
			output := make([]float32, int(tokens*modelWidth))
			invRMS := make([]float32, int(tokens))
			var out C.oct_prom_gemma4e2b_input_rmsnorm_result
			status := int(C.oct_prom_call_gemma4e2b_input_rmsnorm(
				sym,
				C.uintptr_t(handle.ptr),
				floatSlicePointer(input),
				floatSlicePointer(weight),
				floatSlicePointer(output),
				floatSlicePointer(invRMS),
				C.uint64_t(len(input)),
				C.uint64_t(len(weight)),
				C.uint64_t(len(output)),
				C.uint64_t(len(invRMS)),
				C.uint32_t(tokens),
				C.uint32_t(modelWidth),
				C.uint32_t(inputRowStride),
				C.float(epsilon),
				C.uint64_t(inputGeneration),
				C.uint64_t(weightGeneration),
				C.uint64_t(exactSourceHash),
				&out,
			))
			result := reactorGemma4E2BM1InputRMSNormResult{
				StageCode:                    uint32(out.stage),
				DetailCode:                   int(out.detail_code),
				OutputWritten:                out.output_written != 0,
				MatchedInput:                 out.matched_input != 0,
				InputHash:                    uint64(out.input_hash),
				WeightHash:                   uint64(out.weight_hash),
				OutputHash:                   uint64(out.output_hash),
				InvRMSHash:                   uint64(out.inv_rms_hash),
				SubmitCount:                  uint32(out.submit_count),
				FinalReadbackCount:           uint32(out.final_readback_count),
				NoIntermediateReadbackChange: out.no_product_intermediate_readback_change != 0,
				RetainedBytes:                uint64(out.retained_bytes),
				BufferAllocationCount:        uint64(out.buffer_allocation_count),
				BufferReuseCount:             uint64(out.buffer_reuse_count),
				DescriptorUpdateCount:        uint64(out.descriptor_update_count),
				PipelineCreateCount:          uint64(out.pipeline_create_count),
				CommandBufferReuseCount:      uint64(out.command_buffer_reuse_count),
				ReductionGPUNanoseconds:      uint64(out.reduction_gpu_ns),
				FinalReductionGPUNanoseconds: uint64(out.final_reduction_gpu_ns),
				InvRMSGPUNanoseconds:         uint64(out.inv_rms_gpu_ns),
				ApplyGPUNanoseconds:          uint64(out.apply_gpu_ns),
				EndToEndNanoseconds:          uint64(out.end_to_end_ns),
			}
			if status != 0 {
				return nil, nil, result, fmt.Errorf("gemma4e2b input rmsnorm status=%d", status)
			}
			return output, invRMS, result, nil
		}), nil
	case reactorSymbolGemmaRope:
		return reactorGemma4E2BM1Rope(func(handle reactorRuntimeHandle, source, cosine, sine []float32, tokens, heads, headWidth uint32, sourceGeneration, tableGeneration uint64) ([]float32, reactorGemma4E2BM1RopeResult, error) {
			output := make([]float32, int(tokens*heads*headWidth))
			var out C.oct_prom_gemma4e2b_rope_result
			status := int(C.oct_prom_call_gemma4e2b_rope(
				sym, C.uintptr_t(handle.ptr), floatSlicePointer(source), floatSlicePointer(cosine),
				floatSlicePointer(sine), floatSlicePointer(output), C.uint64_t(len(source)),
				C.uint64_t(len(cosine)), C.uint64_t(len(sine)), C.uint64_t(len(output)),
				C.uint32_t(tokens), C.uint32_t(heads), C.uint32_t(headWidth),
				C.uint64_t(sourceGeneration), C.uint64_t(tableGeneration), &out,
			))
			result := reactorGemma4E2BM1RopeResult{
				StageCode: uint32(out.stage), DetailCode: int(out.detail_code),
				OutputWritten: out.output_written != 0, DispatchCount: uint32(out.dispatch_count),
				SourceHash: uint64(out.source_hash), CosineHash: uint64(out.cosine_hash),
				SineHash: uint64(out.sine_hash), OutputHash: uint64(out.output_hash),
				BufferAllocationCount:   uint64(out.buffer_allocation_count),
				BufferReuseCount:        uint64(out.buffer_reuse_count),
				DescriptorUpdateCount:   uint64(out.descriptor_update_count),
				PipelineCreateCount:     uint64(out.pipeline_create_count),
				CommandBufferReuseCount: uint64(out.command_buffer_reuse_count),
			}
			if status != 0 {
				return nil, result, fmt.Errorf("gemma4e2b rope status=%d", status)
			}
			return output, result, nil
		}), nil
	case reactorSymbolGemmaHeadRNRope:
		return reactorGemma4E2BM1HeadRMSNormRope(func(handle reactorRuntimeHandle, input, weight, cosine, sine []float32, tokens, heads, headWidth uint32, epsilon float32, inputGeneration, weightGeneration, tableGeneration, exactSourceHash uint64) ([]float32, reactorGemma4E2BM1HeadRMSNormRopeResult, error) {
			output := make([]float32, int(tokens*heads*headWidth))
			var out C.oct_prom_gemma4e2b_head_rmsnorm_rope_result
			status := int(C.oct_prom_call_gemma4e2b_head_rmsnorm_rope(sym, C.uintptr_t(handle.ptr), floatSlicePointer(input), floatSlicePointer(weight), floatSlicePointer(cosine), floatSlicePointer(sine), floatSlicePointer(output), C.uint64_t(len(input)), C.uint64_t(len(weight)), C.uint64_t(len(cosine)), C.uint64_t(len(sine)), C.uint64_t(len(output)), C.uint32_t(tokens), C.uint32_t(heads), C.uint32_t(headWidth), C.float(epsilon), C.uint64_t(inputGeneration), C.uint64_t(weightGeneration), C.uint64_t(tableGeneration), C.uint64_t(exactSourceHash), &out))
			result := reactorGemma4E2BM1HeadRMSNormRopeResult{StageCode: uint32(out.stage), DetailCode: int(out.detail_code), OutputWritten: out.output_written != 0, DispatchCount: uint32(out.dispatch_count), ResidentSourceBound: out.resident_source_bound != 0, NormalizedReadbackCount: uint32(out.normalized_readback_count), SourceByteRange: uint64(out.source_byte_range), DestinationByteRange: uint64(out.destination_byte_range), InputHash: uint64(out.input_hash), WeightHash: uint64(out.weight_hash), OutputHash: uint64(out.output_hash), BufferAllocationCount: uint64(out.buffer_allocation_count), BufferReuseCount: uint64(out.buffer_reuse_count), DescriptorUpdateCount: uint64(out.descriptor_update_count), PipelineCreateCount: uint64(out.pipeline_create_count), CommandBufferReuseCount: uint64(out.command_buffer_reuse_count), ObservedWeightGeneration: uint64(out.observed_weight_generation), RequestedWeightGeneration: uint64(out.requested_weight_generation)}
			if status != 0 {
				return nil, result, fmt.Errorf("gemma4e2b resident head rmsnorm-rope failed: stage=%d detail=%d", result.StageCode, result.DetailCode)
			}
			return output, result, nil
		}), nil
	case reactorSymbolGemmaAttentionScores:
		return reactorGemma4E2BM1AttentionScores(func(handle reactorRuntimeHandle, query, key, queryWeight, keyWeight, cosine, sine []float32, tokens, queryHeads, keyHeads, headWidth uint32, epsilon, scale float32, preparationOrder uint32, queryGeneration, keyGeneration, queryWeightGeneration, keyWeightGeneration, tableGeneration, querySourceHash, keySourceHash uint64) ([]float32, reactorGemma4E2BM1AttentionScoresResult, error) {
			output := make([]float32, int(queryHeads*tokens*tokens))
			var out C.PrometheusGemma4E2BM1AttentionScoresResult
			status := int(C.oct_prom_call_gemma4e2b_attention_scores(sym, C.uintptr_t(handle.ptr), floatSlicePointer(query), floatSlicePointer(key), floatSlicePointer(queryWeight), floatSlicePointer(keyWeight), floatSlicePointer(cosine), floatSlicePointer(sine), floatSlicePointer(output), C.uint64_t(len(query)), C.uint64_t(len(key)), C.uint64_t(len(queryWeight)), C.uint64_t(len(keyWeight)), C.uint64_t(len(cosine)), C.uint64_t(len(sine)), C.uint64_t(len(output)), C.uint32_t(tokens), C.uint32_t(queryHeads), C.uint32_t(keyHeads), C.uint32_t(headWidth), C.float(epsilon), C.float(scale), C.uint32_t(preparationOrder), C.uint64_t(queryGeneration), C.uint64_t(keyGeneration), C.uint64_t(queryWeightGeneration), C.uint64_t(keyWeightGeneration), C.uint64_t(tableGeneration), C.uint64_t(querySourceHash), C.uint64_t(keySourceHash), &out))
			result := reactorGemma4E2BM1AttentionScoresResult{StageCode: uint32(out.stage), DetailCode: int(out.detail_code), ScoreWritten: out.score_written != 0, PositionalDispatchCount: uint32(out.positional_dispatch_count), ScoreDispatchCount: uint32(out.score_dispatch_count), ScoreReadbackCount: uint32(out.score_readback_count), HostDetourCount: uint32(out.host_detour_count), QuerySlotID: uint32(out.query_slot_id), QuerySlotGeneration: uint32(out.query_slot_generation), KeySlotID: uint32(out.key_slot_id), KeySlotGeneration: uint32(out.key_slot_generation), ScoreSlotID: uint32(out.score_slot_id), ScoreSlotGeneration: uint32(out.score_slot_generation), QueryByteRange: uint64(out.query_byte_range), KeyByteRange: uint64(out.key_byte_range), ScoreByteRange: uint64(out.score_byte_range), ScoreHash: uint64(out.score_hash), BufferAllocationCount: uint64(out.buffer_allocation_count), BufferReuseCount: uint64(out.buffer_reuse_count), DescriptorUpdateCount: uint64(out.descriptor_update_count), PipelineCreateCount: uint64(out.pipeline_create_count), ObservedWeightGeneration: uint64(out.observed_weight_generation), RequestedWeightGeneration: uint64(out.requested_weight_generation)}
			if status != 0 {
				return nil, result, fmt.Errorf("gemma4e2b resident attention scores failed: stage=%d detail=%d positional_dispatches=%d observed_weight_generation=%d requested_weight_generation=%d", result.StageCode, result.DetailCode, result.PositionalDispatchCount, result.ObservedWeightGeneration, result.RequestedWeightGeneration)
			}
			return output, result, nil
		}), nil
	default:
		return nil, fmt.Errorf("unsupported reactor symbol %q", symbol)
	}
}

func (l *dlLibrary) Close() error {
	if l == nil || l.handle == nil {
		return nil
	}
	if rc := C.oct_prom_dlclose(l.handle); rc != 0 {
		return fmt.Errorf("dlclose: %s", dlErrorString())
	}
	l.handle = nil
	return nil
}

func dlErrorString() string {
	err := C.oct_prom_dlerror()
	if err == nil {
		return "unknown dynamic loader error"
	}
	return C.GoString(err)
}

func floatSlicePointer(values []float32) *C.float {
	if len(values) == 0 {
		return nil
	}
	return (*C.float)(unsafe.Pointer(&values[0]))
}

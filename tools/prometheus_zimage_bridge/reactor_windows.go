//go:build windows && cgo

package main

/*
#cgo CFLAGS: -I${SRCDIR}/../../internal/prometheus/native -I${SRCDIR}/../../internal/prometheus/models/zimage-turbo
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include "reactor_api.h"
#include "resolved_audit_schedule.h"
#include "resolved_descriptor.h"

typedef struct oct_prom_zimage_api {
  HMODULE module;
  FARPROC abi_version;
  FARPROC runtime_create;
  FARPROC runtime_destroy;
  FARPROC runtime_probe;
  FARPROC model_create;
  FARPROC model_upload;
  FARPROC model_destroy;
  FARPROC owner_create;
  FARPROC owner_retarget;
  FARPROC owner_prefetch;
  FARPROC owner_activate_prefetch;
  FARPROC evaluation_reset;
  FARPROC noise_execute0;
  FARPROC noise_rebind;
  FARPROC noise_execute_resident;
  FARPROC context_create;
  FARPROC context_rebind;
  FARPROC context_execute0;
  FARPROC context_execute_resident;
  FARPROC main_create;
  FARPROC main_rebind;
  FARPROC main_execute;
  FARPROC main_audit_final;
  FARPROC session_create;
  FARPROC session_get_evidence;
  FARPROC session_capture;
  FARPROC session_compose;
  FARPROC session_destroy;
} oct_prom_zimage_api;

typedef uint32_t (__cdecl *oct_abi_fn)(void);
typedef int (__cdecl *oct_runtime_create_fn)(void*, void**);
typedef int (__cdecl *oct_runtime_destroy_fn)(void*);
typedef int (__cdecl *oct_runtime_probe_fn)(void*, PrometheusCaps*);
typedef int (__cdecl *oct_model_create_fn)(void*, const PrometheusModelBlockCreateRequest*, uint64_t*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_model_upload_fn)(void*, uint64_t, const PrometheusModelBlockWeightUpload*, uint32_t, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_model_destroy_fn)(void*, uint64_t);
typedef int (__cdecl *oct_owner_create_fn)(void*, const PrometheusNoiseRefinerRebindRequest*, uint64_t*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_owner_retarget_fn)(void*, const PrometheusCompiledModelRetargetRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_owner_prefetch_fn)(void*, const PrometheusCompiledModelPrefetchRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_owner_activate_prefetch_fn)(void*, uint64_t, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_evaluation_reset_fn)(void*, uint64_t, PrometheusCompiledModelSessionEvidence*);
typedef int (__cdecl *oct_noise_execute0_fn)(void*, uint64_t, const PrometheusNoiseRefiner0ExecuteRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_noise_rebind_fn)(void*, uint64_t, const PrometheusNoiseRefinerRebindRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_noise_resident_fn)(void*, uint64_t, const PrometheusNoiseRefinerResidentExecuteRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_context_create_fn)(void*, const PrometheusContextRefinerCreateRequest*, uint64_t*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_context_rebind_fn)(void*, uint64_t, const PrometheusContextRefinerRebindRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_context_execute0_fn)(void*, uint64_t, const PrometheusContextRefiner0ExecuteRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_context_resident_fn)(void*, uint64_t, const PrometheusContextRefinerResidentExecuteRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_main_create_fn)(void*, const PrometheusMainTransformerCreateRequest*, uint64_t*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_main_rebind_fn)(void*, uint64_t, const PrometheusMainTransformerRebindRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_main_execute_fn)(void*, uint64_t, const PrometheusMainTransformerExecuteRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_main_audit_fn)(void*, uint64_t, const PrometheusMainTransformerFinalAuditRequest*, PrometheusModelBlockEvidence*);
typedef int (__cdecl *oct_session_create_fn)(void*, const PrometheusCompiledModelSessionCreateRequest*, uint64_t*, PrometheusCompiledModelSessionEvidence*);
typedef int (__cdecl *oct_session_get_evidence_fn)(void*, uint64_t, PrometheusCompiledModelSessionEvidence*);
typedef int (__cdecl *oct_session_capture_fn)(void*, uint64_t, uint64_t, const PrometheusCompiledModelSessionCaptureRequest*, PrometheusCompiledModelSessionEvidence*);
typedef int (__cdecl *oct_session_compose_fn)(void*, uint64_t, const PrometheusCompiledModelSessionComposeRequest*, PrometheusCompiledModelSessionEvidence*);
typedef int (__cdecl *oct_session_destroy_fn)(void*, uint64_t);

static int oct_prom_zimage_load(const char* path, oct_prom_zimage_api* api, DWORD* out_error) {
  if (api == NULL || path == NULL) return 1;
  memset(api, 0, sizeof(*api));
  api->module = LoadLibraryA(path);
  if (api->module == NULL) {
    if (out_error != NULL) *out_error = GetLastError();
    return 1;
  }
#define OCT_LOAD(name, symbol) do { api->name = GetProcAddress(api->module, symbol); if (api->name == NULL) goto missing; } while (0)
  OCT_LOAD(abi_version, "prometheus_reactor_abi_version");
  OCT_LOAD(runtime_create, "prometheus_reactor_runtime_create");
  OCT_LOAD(runtime_destroy, "prometheus_reactor_runtime_destroy");
  OCT_LOAD(runtime_probe, "prometheus_reactor_runtime_probe");
  OCT_LOAD(model_create, "prometheus_reactor_runtime_model_block_create");
  OCT_LOAD(model_upload, "prometheus_reactor_runtime_model_block_upload_weights");
  OCT_LOAD(model_destroy, "prometheus_reactor_runtime_model_block_destroy");
  OCT_LOAD(owner_create, "prometheus_reactor_runtime_compiled_model_owner_create");
  OCT_LOAD(owner_retarget, "prometheus_reactor_runtime_compiled_model_retarget");
  OCT_LOAD(owner_prefetch, "prometheus_reactor_runtime_compiled_model_prefetch");
  OCT_LOAD(owner_activate_prefetch, "prometheus_reactor_runtime_compiled_model_activate_prefetch");
  OCT_LOAD(evaluation_reset, "prometheus_reactor_runtime_compiled_model_evaluation_reset");
  OCT_LOAD(noise_execute0, "prometheus_reactor_runtime_noise_refiner0_execute");
  OCT_LOAD(noise_rebind, "prometheus_reactor_runtime_noise_refiner_rebind");
  OCT_LOAD(noise_execute_resident, "prometheus_reactor_runtime_noise_refiner_execute_resident");
  OCT_LOAD(context_create, "prometheus_reactor_runtime_context_refiner_create");
  OCT_LOAD(context_rebind, "prometheus_reactor_runtime_context_refiner_rebind");
  OCT_LOAD(context_execute0, "prometheus_reactor_runtime_context_refiner0_execute");
  OCT_LOAD(context_execute_resident, "prometheus_reactor_runtime_context_refiner_execute_resident");
  OCT_LOAD(main_create, "prometheus_reactor_runtime_main_transformer_create");
  OCT_LOAD(main_rebind, "prometheus_reactor_runtime_main_transformer_rebind");
  OCT_LOAD(main_execute, "prometheus_reactor_runtime_main_transformer_execute");
  OCT_LOAD(main_audit_final, "prometheus_reactor_runtime_main_transformer_audit_final");
  OCT_LOAD(session_create, "prometheus_reactor_runtime_compiled_model_session_create");
  OCT_LOAD(session_get_evidence, "prometheus_reactor_runtime_compiled_model_session_get_evidence");
  OCT_LOAD(session_capture, "prometheus_reactor_runtime_compiled_model_session_capture_completed");
  OCT_LOAD(session_compose, "prometheus_reactor_runtime_compiled_model_session_compose_joint");
  OCT_LOAD(session_destroy, "prometheus_reactor_runtime_compiled_model_session_destroy");
#undef OCT_LOAD
  if (((oct_abi_fn)api->abi_version)() != 1u) goto missing;
  return 0;
missing:
  if (out_error != NULL) *out_error = GetLastError();
  FreeLibrary(api->module);
  memset(api, 0, sizeof(*api));
  return 1;
}

static void oct_prom_zimage_unload(oct_prom_zimage_api* api) {
  if (api != NULL && api->module != NULL) FreeLibrary(api->module);
  if (api != NULL) memset(api, 0, sizeof(*api));
}

static int oct_prom_runtime_open(oct_prom_zimage_api* api, void** out_runtime, PrometheusCaps* out_caps) {
  int status;
  if (api == NULL || out_runtime == NULL || out_caps == NULL) return PROM_ERROR;
  *out_runtime = NULL;
  memset(out_caps, 0, sizeof(*out_caps));
  status = ((oct_runtime_create_fn)api->runtime_create)(NULL, out_runtime);
  if (status != PROM_OK || *out_runtime == NULL) return PROM_ERROR;
  status = ((oct_runtime_probe_fn)api->runtime_probe)(*out_runtime, out_caps);
  if (status != PROM_OK || out_caps->available == 0u) {
    ((oct_runtime_destroy_fn)api->runtime_destroy)(*out_runtime);
    *out_runtime = NULL;
    return PROM_ERROR;
  }
  return PROM_OK;
}

static int oct_prom_runtime_close(oct_prom_zimage_api* api, void* runtime) {
  return runtime == NULL ? PROM_OK : ((oct_runtime_destroy_fn)api->runtime_destroy)(runtime);
}

static int oct_prom_session_create(oct_prom_zimage_api* api, void* runtime, uint32_t profile, uint32_t attention_route, uint64_t* out_session,
                                   PrometheusCompiledModelSessionEvidence* evidence) {
  PrometheusCompiledModelSessionCreateRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.execution_profile = profile;
  request.main_attention_route_policy = attention_route;
  return ((oct_session_create_fn)api->session_create)(runtime, &request, out_session, evidence);
}

static int oct_prom_session_get_evidence(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                         PrometheusCompiledModelSessionEvidence* evidence) {
  return ((oct_session_get_evidence_fn)api->session_get_evidence)(runtime, session_id, evidence);
}

static int oct_prom_session_capture(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                    uint64_t block_id, uint64_t generation,
                                    PrometheusCompiledModelSessionEvidence* evidence) {
  PrometheusCompiledModelSessionCaptureRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.source_output_generation = generation;
  return ((oct_session_capture_fn)api->session_capture)(runtime, session_id, block_id, &request, evidence);
}

static int oct_prom_session_compose(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                    uint64_t image_generation, uint64_t context_generation,
                                    PrometheusCompiledModelSessionEvidence* evidence) {
  PrometheusCompiledModelSessionComposeRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.required_image_generation = image_generation;
  request.required_context_generation = context_generation;
  return ((oct_session_compose_fn)api->session_compose)(runtime, session_id, &request, evidence);
}

static int oct_prom_session_destroy(oct_prom_zimage_api* api, void* runtime, uint64_t session_id) {
  return ((oct_session_destroy_fn)api->session_destroy)(runtime, session_id);
}

static int oct_prom_owner_create(oct_prom_zimage_api* api, void* runtime,
                                 const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                 uint64_t* out_block, PrometheusModelBlockEvidence* evidence) {
  PrometheusNoiseRefinerRebindRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = 0u;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_owner_create_fn)api->owner_create)(runtime, &request, out_block, evidence);
}

static int oct_prom_owner_retarget(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                   uint32_t local_block_id, const PrometheusModelBlockWeightUpload* uploads,
                                   uint32_t upload_count, PrometheusModelBlockEvidence* evidence) {
  PrometheusCompiledModelRetargetRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.session_identity = session_id;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.model_local_block_id = local_block_id;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_owner_retarget_fn)api->owner_retarget)(runtime, &request, evidence);
}

static int oct_prom_owner_prefetch(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                   uint32_t local_block_id, const PrometheusModelBlockWeightUpload* uploads,
                                   uint32_t upload_count, PrometheusModelBlockEvidence* evidence) {
  PrometheusCompiledModelPrefetchRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.session_identity = session_id;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.model_local_block_id = local_block_id;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_owner_prefetch_fn)api->owner_prefetch)(runtime, &request, evidence);
}

static int oct_prom_owner_activate_prefetch(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                            PrometheusModelBlockEvidence* evidence) {
  return ((oct_owner_activate_prefetch_fn)api->owner_activate_prefetch)(runtime, session_id, evidence);
}

static int oct_prom_evaluation_reset(oct_prom_zimage_api* api, void* runtime, uint64_t session_id,
                                     PrometheusCompiledModelSessionEvidence* evidence) {
  return ((oct_evaluation_reset_fn)api->evaluation_reset)(runtime, session_id, evidence);
}

static int oct_prom_noise_create(oct_prom_zimage_api* api, void* runtime,
                                 const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                 uint64_t* out_block, PrometheusModelBlockEvidence* evidence) {
  PrometheusModelBlockCreateRequest request;
  uint32_t i;
  int status;
  if (uploads == NULL || upload_count != PROM_MODEL_BLOCK_MAX_WEIGHTS) return PROM_ERROR;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.assembly_family = PROM_NOISE_REFINER_FAMILY_Z_IMAGE_TURBO;
  request.parameter_set = PROM_NOISE_REFINER_PARAMETER_SET_0;
  request.parameter_set_aggregate_identity = 0xa1ba526898a2a752ull;
  request.model_contract_identity = 0x101u;
  request.weight_identity = 0x102u;
  request.shader_portfolio_identity = 0x103u;
  request.precision_policy_identity = 0x104u;
  request.capability_route_identity = 0x105u;
  request.memory_ceiling_bytes = 512ull * 1024ull * 1024ull;
  request.external_input_bytes = 1024ull * 3840ull * sizeof(uint16_t);
  request.audit_bytes = PROM_ZIMAGE_TURBO_AUDIT_ARENA_BYTES;
  request.shader_id = 24u;
  request.weight_count = upload_count;
  request.step_count = PROM_MODEL_BLOCK_MAX_STEPS;
  for (i = 0u; i < request.step_count; ++i) request.steps[i] = i + 1u;
  for (i = 0u; i < upload_count; ++i) {
    request.weights[i].content_identity = uploads[i].content_identity;
    request.weights[i].layout_identity = uploads[i].layout_identity;
    request.weights[i].byte_count = uploads[i].byte_count;
  }
  status = ((oct_model_create_fn)api->model_create)(runtime, &request, out_block, evidence);
  if (status != PROM_OK) return status;
  status = ((oct_model_upload_fn)api->model_upload)(runtime, *out_block, uploads, upload_count, evidence);
  if (status != PROM_OK) {
    ((oct_model_destroy_fn)api->model_destroy)(runtime, *out_block);
    *out_block = 0u;
  }
  return status;
}

static int oct_prom_noise_execute0(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                   const void* image, uint64_t image_bytes,
                                   const void* timestep, uint64_t timestep_bytes,
                                   uint64_t input_identity, uint64_t timestep_identity,
                                   uint64_t output_identity, PrometheusModelBlockEvidence* evidence) {
  PrometheusNoiseRefiner0ExecuteRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_input_bf16 = image;
  request.timestep_bf16 = timestep;
  request.model_input_bytes = image_bytes;
  request.timestep_bytes = timestep_bytes;
  request.input_identity = input_identity;
  request.timestep_identity = timestep_identity;
  request.output_identity = output_identity;
  return ((oct_noise_execute0_fn)api->noise_execute0)(runtime, block_id, &request, evidence);
}

static int oct_prom_noise_rebind(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                 uint32_t model_local_block_id,
                                 const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                 PrometheusModelBlockEvidence* evidence) {
  PrometheusNoiseRefinerRebindRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = model_local_block_id;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_noise_rebind_fn)api->noise_rebind)(runtime, block_id, &request, evidence);
}

static int oct_prom_noise_execute_resident(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                           uint64_t input_generation, uint64_t output_identity,
                                           PrometheusModelBlockEvidence* evidence) {
  PrometheusNoiseRefinerResidentExecuteRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.input_generation = input_generation;
  request.output_identity = output_identity;
  return ((oct_noise_resident_fn)api->noise_execute_resident)(runtime, block_id, &request, evidence);
}

static int oct_prom_context_create(oct_prom_zimage_api* api, void* runtime,
                                   const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                   uint64_t* out_block, PrometheusModelBlockEvidence* evidence) {
  PrometheusContextRefinerCreateRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = 0u;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_context_create_fn)api->context_create)(runtime, &request, out_block, evidence);
}

static int oct_prom_context_execute0(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                     const float* context, uint64_t context_bytes,
                                     uint64_t input_identity, uint64_t output_identity,
                                     PrometheusModelBlockEvidence* evidence) {
  PrometheusContextRefiner0ExecuteRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.context_input = context;
  request.context_input_bytes = context_bytes;
  request.input_identity = input_identity;
  request.output_identity = output_identity;
  return ((oct_context_execute0_fn)api->context_execute0)(runtime, block_id, &request, evidence);
}

static int oct_prom_context_rebind(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                   const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                   PrometheusModelBlockEvidence* evidence) {
  PrometheusContextRefinerRebindRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = 1u;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_context_rebind_fn)api->context_rebind)(runtime, block_id, &request, evidence);
}

static int oct_prom_context_execute_resident(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                             uint64_t input_generation, uint64_t output_identity,
                                             PrometheusModelBlockEvidence* evidence) {
  PrometheusContextRefinerResidentExecuteRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.input_generation = input_generation;
  request.output_identity = output_identity;
  return ((oct_context_resident_fn)api->context_execute_resident)(runtime, block_id, &request, evidence);
}

static int oct_prom_main_create(oct_prom_zimage_api* api, void* runtime,
                                const PrometheusModelBlockWeightUpload* uploads, uint32_t upload_count,
                                uint64_t* out_block, PrometheusModelBlockEvidence* evidence) {
  PrometheusMainTransformerCreateRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = 0u;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_main_create_fn)api->main_create)(runtime, &request, out_block, evidence);
}

static int oct_prom_main_rebind(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                uint32_t layer, const PrometheusModelBlockWeightUpload* uploads,
                                uint32_t upload_count, PrometheusModelBlockEvidence* evidence) {
  PrometheusMainTransformerRebindRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.model_local_block_id = layer;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.upload_count = upload_count;
  request.uploads = uploads;
  return ((oct_main_rebind_fn)api->main_rebind)(runtime, block_id, &request, evidence);
}

static int oct_prom_main_execute(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                 uint64_t session_id, uint32_t layer,
                                 uint64_t image_generation, uint64_t context_generation,
                                 uint64_t joint_generation, const void* timestep,
                                 uint64_t timestep_bytes, uint64_t timestep_identity,
                                 uint64_t output_identity, PrometheusModelBlockEvidence* evidence) {
  PrometheusMainTransformerExecuteRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.session_identity = session_id;
  request.lock_identity = PROM_ZIMAGE_TURBO_LOCK_ID;
  request.model_local_block_id = layer;
  request.resident_chain_mode = 1u;
  request.required_image_generation = image_generation;
  request.required_context_generation = context_generation;
  request.required_joint_generation = joint_generation;
  request.timestep_bf16 = timestep;
  request.timestep_bytes = timestep_bytes;
  request.timestep_identity = timestep_identity;
  request.output_identity = output_identity;
  return ((oct_main_execute_fn)api->main_execute)(runtime, block_id, &request, evidence);
}

static int oct_prom_main_audit_final(oct_prom_zimage_api* api, void* runtime, uint64_t block_id,
                                     uint64_t generation, uint64_t output_identity,
                                     float* output, uint64_t output_elements,
                                     PrometheusModelBlockEvidence* evidence) {
  PrometheusMainTransformerFinalAuditRequest request;
  memset(&request, 0, sizeof(request));
  request.struct_size = sizeof(request);
  request.required_output_generation = generation;
  request.output_identity = output_identity;
  request.output = output;
  request.output_element_capacity = output_elements;
  return ((oct_main_audit_fn)api->main_audit_final)(runtime, block_id, &request, evidence);
}

static int oct_prom_model_destroy(oct_prom_zimage_api* api, void* runtime, uint64_t block_id) {
  return ((oct_model_destroy_fn)api->model_destroy)(runtime, block_id);
}
*/
import "C"

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
	"unsafe"
)

type reactorDLL struct {
	api            C.oct_prom_zimage_api
	runtime        unsafe.Pointer
	sessionID      uint64
	ownerID        uint64
	profile        C.uint32_t
	attentionRoute C.uint32_t
}

type modelExecutionProfile uint32

type mainAttentionRoute uint32

const (
	minimumMemoryProfile modelExecutionProfile = modelExecutionProfile(C.PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY)
	prefetchProfile      modelExecutionProfile = modelExecutionProfile(C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH)
)

func selectedExecutionProfile(requested uint32) (modelExecutionProfile, error) {
	switch modelExecutionProfile(requested) {
	case minimumMemoryProfile, prefetchProfile:
		return modelExecutionProfile(requested), nil
	default:
		return 0, fmt.Errorf("execution_profile=%d must be MinimumMemory (1) or Prefetch (2)", requested)
	}
}

const (
	mainAttentionAuto            mainAttentionRoute = mainAttentionRoute(C.PROM_MAIN_ATTENTION_ROUTE_AUTO)
	mainAttentionSerialCanonical mainAttentionRoute = mainAttentionRoute(C.PROM_MAIN_ATTENTION_ROUTE_SERIAL_CANONICAL)
	mainAttentionSubgroupOwned32 mainAttentionRoute = mainAttentionRoute(C.PROM_MAIN_ATTENTION_ROUTE_SUBGROUP_OWNED32)
	mainAttentionBuiltinTopology mainAttentionRoute = mainAttentionRoute(C.PROM_MAIN_ATTENTION_ROUTE_BUILTIN_TOPOLOGY)
)

func selectedMainAttentionRoute(requested uint32) (mainAttentionRoute, error) {
	switch mainAttentionRoute(requested) {
	case mainAttentionAuto, mainAttentionSerialCanonical, mainAttentionSubgroupOwned32, mainAttentionBuiltinTopology:
		return mainAttentionRoute(requested), nil
	default:
		return 0, fmt.Errorf("main_attention_route_policy=%d must be Auto (1), SerialCanonical (2), SubgroupOwned32 (3), or BuiltinTopology (4)", requested)
	}
}

type loadedUploads struct {
	pointer    *C.PrometheusModelBlockWeightUpload
	allocation unsafe.Pointer
	buffers    []unsafe.Pointer
	byteCount  uint64
}

type prefetchResult struct {
	evidence C.PrometheusModelBlockEvidence
	duration uint64
	started  time.Time
	ended    time.Time
	err      error
}

func (reactor *reactorDLL) prefetch(blockID uint32, payload loadedUploads) <-chan prefetchResult {
	result := make(chan prefetchResult, 1)
	if reactor.profile != C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH {
		result <- prefetchResult{}
		return result
	}
	go func() {
		var evidence C.PrometheusModelBlockEvidence
		start := time.Now()
		status := C.oct_prom_owner_prefetch(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), C.uint32_t(blockID), payload.pointer, C.uint32_t(len(payload.buffers)), &evidence)
		ended := time.Now()
		entry := prefetchResult{evidence: evidence, duration: uint64(ended.Sub(start)), started: start, ended: ended}
		if status != C.PROM_OK {
			entry.err = fmt.Errorf("prefetch block %d: detail=%d", blockID, int32(evidence.detail_code))
		}
		result <- entry
	}()
	return result
}

func (reactor *reactorDLL) activatePrefetch(result <-chan prefetchResult) (C.PrometheusModelBlockEvidence, prefetchResult, uint64, error) {
	entry := <-result
	if entry.err != nil {
		return entry.evidence, entry, entry.duration, entry.err
	}
	if reactor.profile != C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH {
		return entry.evidence, entry, entry.duration, nil
	}
	start := time.Now()
	if C.oct_prom_owner_activate_prefetch(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), &entry.evidence) != C.PROM_OK {
		return entry.evidence, entry, entry.duration + uint64(time.Since(start)), fmt.Errorf("activate prefetched block: detail=%d", int32(entry.evidence.detail_code))
	}
	return entry.evidence, entry, entry.duration + uint64(time.Since(start)), nil
}

func recordPrefetchOverlap(metrics *runMetrics, entry prefetchResult, computeStart, computeEnd time.Time) {
	if entry.started.IsZero() || entry.ended.IsZero() {
		return
	}
	metrics.prefetchCount++
	metrics.prefetchTransferNS += entry.duration
	start := entry.started
	if computeStart.After(start) {
		start = computeStart
	}
	end := entry.ended
	if computeEnd.Before(end) {
		end = computeEnd
	}
	if end.After(start) {
		metrics.prefetchOverlapNS += uint64(end.Sub(start))
	}
	if entry.ended.After(computeEnd) {
		metrics.prefetchWaitNS += uint64(entry.ended.Sub(computeEnd))
	}
}

// cachedPayload is owned by one bridge session. Its C allocations intentionally
// remain live until session teardown: every package is immutable, lock-validated
// before caching, and reactor calls synchronously consume the upload pointers.
// This is a host-residency optimization only; it never creates another device
// weight window or changes the lock-derived execution order.
type cachedPayload struct {
	noise   [2]loadedUploads
	context [2]loadedUploads
	main    [30]loadedUploads
	bytes   uint64
}

func openReactor(path string, requestedProfile uint32, requestedAttentionRoute uint32) (*reactorDLL, error) {
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	profile, err := selectedExecutionProfile(requestedProfile)
	if err != nil {
		return nil, err
	}
	route, err := selectedMainAttentionRoute(requestedAttentionRoute)
	if err != nil {
		return nil, err
	}
	reactor := &reactorDLL{profile: C.uint32_t(profile), attentionRoute: C.uint32_t(route)}
	var winError C.DWORD
	if C.oct_prom_zimage_load(cPath, &reactor.api, &winError) != 0 {
		return nil, fmt.Errorf("load Prometheus reactor %s: Win32 error %d", path, uint32(winError))
	}
	var caps C.PrometheusCaps
	if C.oct_prom_runtime_open(&reactor.api, &reactor.runtime, &caps) != C.PROM_OK {
		C.oct_prom_zimage_unload(&reactor.api)
		return nil, fmt.Errorf("create Prometheus Vulkan runtime")
	}
	var evidence C.PrometheusCompiledModelSessionEvidence
	if C.oct_prom_session_create(&reactor.api, reactor.runtime, reactor.profile, reactor.attentionRoute, (*C.uint64_t)(&reactor.sessionID), &evidence) != C.PROM_OK {
		C.oct_prom_runtime_close(&reactor.api, reactor.runtime)
		C.oct_prom_zimage_unload(&reactor.api)
		return nil, fmt.Errorf("create lock-resolved compiled-model session: detail=%d", int32(evidence.detail_code))
	}
	return reactor, nil
}

func (reactor *reactorDLL) close() error {
	if reactor == nil {
		return nil
	}
	var first error
	if reactor.runtime != nil && reactor.ownerID != 0 {
		if C.oct_prom_model_destroy(&reactor.api, reactor.runtime, C.uint64_t(reactor.ownerID)) != C.PROM_OK {
			first = fmt.Errorf("destroy persistent compiled-model owner")
		}
		reactor.ownerID = 0
	}
	if reactor.runtime != nil && reactor.sessionID != 0 {
		if C.oct_prom_session_destroy(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID)) != C.PROM_OK {
			first = fmt.Errorf("destroy compiled-model session")
		}
		reactor.sessionID = 0
	}
	if reactor.runtime != nil {
		if C.oct_prom_runtime_close(&reactor.api, reactor.runtime) != C.PROM_OK && first == nil {
			first = fmt.Errorf("destroy Prometheus runtime")
		}
		reactor.runtime = nil
	}
	C.oct_prom_zimage_unload(&reactor.api)
	return first
}

func cachePayload(payload validatedPayload) (*cachedPayload, error) {
	cache := &cachedPayload{}
	for index := range payload.noise {
		loaded, err := loadUploads(payload.noise[index])
		if err != nil {
			cache.free()
			return nil, fmt.Errorf("cache %s: %w", payload.noise[index].name, err)
		}
		cache.noise[index] = loaded
		cache.bytes += loaded.byteCount
	}
	for index := range payload.context {
		loaded, err := loadUploads(payload.context[index])
		if err != nil {
			cache.free()
			return nil, fmt.Errorf("cache %s: %w", payload.context[index].name, err)
		}
		cache.context[index] = loaded
		cache.bytes += loaded.byteCount
	}
	for index := range payload.main {
		loaded, err := loadUploads(payload.main[index])
		if err != nil {
			cache.free()
			return nil, fmt.Errorf("cache %s: %w", payload.main[index].name, err)
		}
		cache.main[index] = loaded
		cache.bytes += loaded.byteCount
	}
	return cache, nil
}

func (cache *cachedPayload) free() {
	if cache == nil {
		return
	}
	for index := range cache.noise {
		cache.noise[index].free()
	}
	for index := range cache.context {
		cache.context[index].free()
	}
	for index := range cache.main {
		cache.main[index].free()
	}
	*cache = cachedPayload{}
}

func loadUploads(block payloadBlock) (loadedUploads, error) {
	count := len(block.tensors)
	allocation := C.calloc(C.size_t(count), C.size_t(C.sizeof_PrometheusModelBlockWeightUpload))
	if allocation == nil {
		return loadedUploads{}, fmt.Errorf("allocate %s upload declarations", block.name)
	}
	uploads := unsafe.Slice((*C.PrometheusModelBlockWeightUpload)(allocation), count)
	loaded := loadedUploads{
		pointer:    (*C.PrometheusModelBlockWeightUpload)(allocation),
		allocation: allocation,
		buffers:    make([]unsafe.Pointer, 0, count),
	}
	for index, tensor := range block.tensors {
		buffer := C.malloc(C.size_t(tensor.byteCount))
		if buffer == nil {
			loaded.free()
			return loadedUploads{}, fmt.Errorf("allocate %s tensor %d (%d bytes)", block.name, index, tensor.byteCount)
		}
		loaded.buffers = append(loaded.buffers, buffer)
		bytes := unsafe.Slice((*byte)(buffer), int(tensor.byteCount))
		file, err := os.Open(tensor.path)
		if err != nil {
			loaded.free()
			return loadedUploads{}, err
		}
		_, readErr := io.ReadFull(file, bytes)
		closeErr := file.Close()
		if readErr != nil || closeErr != nil {
			loaded.free()
			if readErr != nil {
				return loadedUploads{}, fmt.Errorf("read %s: %w", tensor.path, readErr)
			}
			return loadedUploads{}, fmt.Errorf("close %s: %w", tensor.path, closeErr)
		}
		sum := sha256.Sum256(bytes)
		if digest := hex.EncodeToString(sum[:]); digest != tensor.sha256 {
			loaded.free()
			return loadedUploads{}, fmt.Errorf("payload changed after session validation: %s", tensor.path)
		}
		uploads[index].binding_index = C.uint32_t(index)
		uploads[index].bytes = buffer
		uploads[index].byte_count = C.uint64_t(tensor.byteCount)
		uploads[index].content_identity = C.uint64_t(tensor.contentIdentity)
		uploads[index].layout_identity = C.uint64_t(tensor.layoutIdentity)
		loaded.byteCount += tensor.byteCount
	}
	return loaded, nil
}

func (uploads *loadedUploads) free() {
	if uploads == nil {
		return
	}
	for _, buffer := range uploads.buffers {
		C.free(buffer)
	}
	if uploads.allocation != nil {
		C.free(uploads.allocation)
	}
	*uploads = loadedUploads{}
}

func (reactor *reactorDLL) prepareContext(payload [2]loadedUploads, context unsafe.Pointer, identity uint64) (uint64, runMetrics, error) {
	var metrics runMetrics
	var evidence C.PrometheusModelBlockEvidence
	blockID := C.uint64_t(reactor.ownerID)
	rebindStart := time.Now()
	status := C.oct_prom_owner_retarget(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), 0, payload[0].pointer, C.uint32_t(len(payload[0].buffers)), &evidence)
	rebindDuration := uint64(time.Since(rebindStart))
	metrics.parameterRebindNS += rebindDuration
	metrics.stageRebindNS[2] += rebindDuration
	metrics.uploadedWeightBytes += payload[0].byteCount
	metrics.stageUploadedBytes[2] += payload[0].byteCount
	if status != C.PROM_OK {
		return 0, metrics, fmt.Errorf("retarget ContextRefiner0: detail=%d", int32(evidence.detail_code))
	}
	nextPrefetch := reactor.prefetch(1, payload[1])
	computeStart := time.Now()
	if C.oct_prom_context_execute0(&reactor.api, reactor.runtime, blockID, (*C.float)(context), C.uint64_t(contextFP32Bytes), C.uint64_t(identity), C.uint64_t(identityFromText("context_refiner.0 output")), &evidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("execute ContextRefiner0: detail=%d", int32(evidence.detail_code))
	}
	computeEnd := time.Now()
	metrics.modelExecutionNS += uint64(evidence.last_execution_ns)
	metrics.stageExecutionNS[2] += uint64(evidence.last_execution_ns)
	metrics.stageGPUExecutionNS[2] += uint64(evidence.gpu_compute_ns)
	inputGeneration := uint64(evidence.output_generation)
	if reactor.profile == C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH {
		var activateErr error
		var prefetchEntry prefetchResult
		evidence, prefetchEntry, rebindDuration, activateErr = reactor.activatePrefetch(nextPrefetch)
		recordPrefetchOverlap(&metrics, prefetchEntry, computeStart, computeEnd)
		if activateErr != nil {
			return 0, metrics, activateErr
		}
		status = C.PROM_OK
	} else {
		rebindStart = time.Now()
		status = C.oct_prom_owner_retarget(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), 1, payload[1].pointer, C.uint32_t(len(payload[1].buffers)), &evidence)
		rebindDuration = uint64(time.Since(rebindStart))
	}
	metrics.parameterRebindNS += rebindDuration
	metrics.stageRebindNS[3] += rebindDuration
	metrics.uploadedWeightBytes += payload[1].byteCount
	metrics.stageUploadedBytes[3] += payload[1].byteCount
	if status != C.PROM_OK {
		return 0, metrics, fmt.Errorf("retarget ContextRefiner1: detail=%d", int32(evidence.detail_code))
	}
	if C.oct_prom_context_execute_resident(&reactor.api, reactor.runtime, blockID, C.uint64_t(inputGeneration), C.uint64_t(identityFromText("context_refiner.1 output")), &evidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("execute ContextRefiner1: detail=%d", int32(evidence.detail_code))
	}
	metrics.modelExecutionNS += uint64(evidence.last_execution_ns)
	metrics.stageExecutionNS[3] += uint64(evidence.last_execution_ns)
	metrics.stageGPUExecutionNS[3] += uint64(evidence.gpu_compute_ns)
	var sessionEvidence C.PrometheusCompiledModelSessionEvidence
	if C.oct_prom_session_capture(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), blockID, evidence.output_generation, &sessionEvidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("capture PreparedContext: detail=%d", int32(sessionEvidence.detail_code))
	}
	return uint64(sessionEvidence.prepared_context_generation), metrics, nil
}

func (reactor *reactorDLL) prepareImage(payload [2]loadedUploads, image, timestep unsafe.Pointer, inputIdentity, timestepIdentity uint64) (uint64, runMetrics, error) {
	var metrics runMetrics
	var evidence C.PrometheusModelBlockEvidence
	var blockID C.uint64_t
	var status C.int
	if reactor.ownerID == 0 {
		status = C.oct_prom_owner_create(&reactor.api, reactor.runtime, payload[0].pointer, C.uint32_t(len(payload[0].buffers)), &blockID, &evidence)
		if status == C.PROM_OK {
			reactor.ownerID = uint64(blockID)
			var sessionEvidence C.PrometheusCompiledModelSessionEvidence
			if C.oct_prom_session_get_evidence(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), &sessionEvidence) != C.PROM_OK {
				return 0, metrics, fmt.Errorf("inspect selected execution profile: detail=%d", int32(sessionEvidence.detail_code))
			}
			switch sessionEvidence.selected_execution_profile {
			case C.PROM_MODEL_EXECUTION_PROFILE_MINIMUM_MEMORY, C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH:
				reactor.profile = sessionEvidence.selected_execution_profile
			default:
				return 0, metrics, fmt.Errorf("native selected unknown execution profile %d", uint32(sessionEvidence.selected_execution_profile))
			}
		}
	} else {
		var sessionEvidence C.PrometheusCompiledModelSessionEvidence
		if C.oct_prom_evaluation_reset(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), &sessionEvidence) != C.PROM_OK {
			return 0, metrics, fmt.Errorf("reset persistent owner: detail=%d", int32(sessionEvidence.detail_code))
		}
		blockID = C.uint64_t(reactor.ownerID)
		rebindStart := time.Now()
		status = C.oct_prom_owner_retarget(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), 0, payload[0].pointer, C.uint32_t(len(payload[0].buffers)), &evidence)
		rebindDuration := uint64(time.Since(rebindStart))
		metrics.parameterRebindNS += rebindDuration
		metrics.stageRebindNS[0] += rebindDuration
	}
	metrics.uploadedWeightBytes += payload[0].byteCount
	metrics.stageUploadedBytes[0] += payload[0].byteCount
	if status != C.PROM_OK {
		return 0, metrics, fmt.Errorf("bind NoiseRefiner0: detail=%d", int32(evidence.detail_code))
	}
	nextPrefetch := reactor.prefetch(1, payload[1])
	outputIdentity := identityFromText(fmt.Sprintf("noise0:%016x:%016x", inputIdentity, timestepIdentity))
	computeStart := time.Now()
	if C.oct_prom_noise_execute0(&reactor.api, reactor.runtime, blockID, image, C.uint64_t(imageBF16Bytes), timestep, C.uint64_t(timestepBF16Bytes), C.uint64_t(inputIdentity), C.uint64_t(timestepIdentity), C.uint64_t(outputIdentity), &evidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("execute NoiseRefiner0: detail=%d", int32(evidence.detail_code))
	}
	computeEnd := time.Now()
	metrics.modelExecutionNS += uint64(evidence.last_execution_ns)
	metrics.stageExecutionNS[0] += uint64(evidence.last_execution_ns)
	metrics.stageGPUExecutionNS[0] += uint64(evidence.gpu_compute_ns)
	inputGeneration := uint64(evidence.output_generation)
	var rebindDuration uint64
	if reactor.profile == C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH {
		var activateErr error
		var prefetchEntry prefetchResult
		evidence, prefetchEntry, rebindDuration, activateErr = reactor.activatePrefetch(nextPrefetch)
		recordPrefetchOverlap(&metrics, prefetchEntry, computeStart, computeEnd)
		if activateErr != nil {
			return 0, metrics, activateErr
		}
		status = C.PROM_OK
	} else {
		rebindStart := time.Now()
		status = C.oct_prom_owner_retarget(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), 1, payload[1].pointer, C.uint32_t(len(payload[1].buffers)), &evidence)
		rebindDuration = uint64(time.Since(rebindStart))
	}
	metrics.parameterRebindNS += rebindDuration
	metrics.stageRebindNS[1] += rebindDuration
	metrics.uploadedWeightBytes += payload[1].byteCount
	metrics.stageUploadedBytes[1] += payload[1].byteCount
	if status != C.PROM_OK {
		return 0, metrics, fmt.Errorf("retarget NoiseRefiner1: detail=%d", int32(evidence.detail_code))
	}
	if C.oct_prom_noise_execute_resident(&reactor.api, reactor.runtime, blockID, C.uint64_t(inputGeneration), C.uint64_t(identityFromText("noise_refiner.1 output")), &evidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("execute NoiseRefiner1: detail=%d", int32(evidence.detail_code))
	}
	metrics.modelExecutionNS += uint64(evidence.last_execution_ns)
	metrics.stageExecutionNS[1] += uint64(evidence.last_execution_ns)
	metrics.stageGPUExecutionNS[1] += uint64(evidence.gpu_compute_ns)
	var sessionEvidence C.PrometheusCompiledModelSessionEvidence
	if C.oct_prom_session_capture(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), blockID, evidence.output_generation, &sessionEvidence) != C.PROM_OK {
		return 0, metrics, fmt.Errorf("capture PreparedImage: detail=%d", int32(sessionEvidence.detail_code))
	}
	return uint64(sessionEvidence.prepared_image_generation), metrics, nil
}

func (reactor *reactorDLL) compose(imageGeneration, contextGeneration uint64) (uint64, error) {
	var evidence C.PrometheusCompiledModelSessionEvidence
	if C.oct_prom_session_compose(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), C.uint64_t(imageGeneration), C.uint64_t(contextGeneration), &evidence) != C.PROM_OK {
		return 0, fmt.Errorf("compose JointWorking: detail=%d", int32(evidence.detail_code))
	}
	return uint64(evidence.joint_generation), nil
}

func recordMainLayerTrace(trace *mainLayerTrace, evidence *C.PrometheusModelBlockEvidence, layer int, evaluationStart, cpuBegin, cpuEnd time.Time) {
	trace.correlationID = identityFromText(fmt.Sprintf("dvt2-m3:main:%d:%d", layer, uint64(evidence.output_generation)))
	trace.cpuBeginNS = uint64(cpuBegin.Sub(evaluationStart))
	trace.cpuEndNS = uint64(cpuEnd.Sub(evaluationStart))
	trace.parameterGeneration = uint64(evidence.binding_generation)
	trace.executionGeneration = uint64(evidence.output_generation)
	trace.activeWeightWindow = uint32(evidence.active_weight_window)
	trace.gpuTotalBeginTick = uint64(evidence.gpu_total_begin_tick)
	trace.gpuTotalEndTick = uint64(evidence.gpu_total_end_tick)
	trace.gpuTotalNS = uint64(evidence.gpu_total_ns)
	trace.gpuComputeBeginTick = uint64(evidence.gpu_compute_begin_tick)
	trace.gpuComputeEndTick = uint64(evidence.gpu_compute_end_tick)
	trace.gpuComputeNS = uint64(evidence.gpu_compute_ns)
	trace.gpuIngressTransferNS = uint64(evidence.gpu_ingress_transfer_ns)
	trace.gpuJointCopyNS = uint64(evidence.gpu_joint_copy_ns)
	beginTicks := unsafe.Slice((*uint64)(unsafe.Pointer(&evidence.main_stage_gpu_begin_tick[0])), mainStageCount)
	endTicks := unsafe.Slice((*uint64)(unsafe.Pointer(&evidence.main_stage_gpu_end_tick[0])), mainStageCount)
	durations := unsafe.Slice((*uint64)(unsafe.Pointer(&evidence.main_stage_gpu_ns[0])), mainStageCount)
	for stage := 0; stage < mainStageCount; stage++ {
		trace.stageGPUBeginTick[stage] = beginTicks[stage]
		trace.stageGPUEndTick[stage] = endTicks[stage]
		trace.stageGPUNS[stage] = durations[stage]
	}
	trace.activeTargetValidationNS = uint64(evidence.last_active_target_validation_ns)
	trace.commandResetNS = uint64(evidence.last_command_reset_ns)
	trace.commandBeginNS = uint64(evidence.last_command_begin_ns)
	trace.commandRecordNS = uint64(evidence.last_command_record_ns)
	trace.commandEndNS = uint64(evidence.last_command_end_ns)
	trace.queueSubmitNS = uint64(evidence.last_queue_submit_ns)
	trace.fenceWaitNS = uint64(evidence.last_fence_wait_ns)
	trace.descriptorUpdateNS = uint64(evidence.last_descriptor_update_ns)
	trace.stagingMemcpyNS = uint64(evidence.last_staging_memcpy_ns)
	trace.counters = nativeCallCounters{
		createBuffer: uint64(evidence.vk_create_buffer_count), destroyBuffer: uint64(evidence.vk_destroy_buffer_count),
		allocateMemory: uint64(evidence.vk_allocate_memory_count), freeMemory: uint64(evidence.vk_free_memory_count),
		createShaderModule: uint64(evidence.vk_create_shader_module_count), destroyShaderModule: uint64(evidence.vk_destroy_shader_module_count),
		createPipelines: uint64(evidence.vk_create_compute_pipelines_count), allocateDescriptorSets: uint64(evidence.vk_allocate_descriptor_sets_count),
		updateDescriptorSets: uint64(evidence.vk_update_descriptor_sets_count), createCommandPool: uint64(evidence.vk_create_command_pool_count),
		allocateCommandBuffers: uint64(evidence.vk_allocate_command_buffers_count), resetCommandBuffer: uint64(evidence.vk_reset_command_buffer_count),
		queueSubmit: uint64(evidence.vk_queue_submit_count), fenceWait: uint64(evidence.vk_fence_wait_count), timelineWait: uint64(evidence.vk_timeline_wait_count),
		mapMemory: uint64(evidence.vk_map_memory_count), unmapMemory: uint64(evidence.vk_unmap_memory_count),
		flush: uint64(evidence.vk_flush_count), invalidate: uint64(evidence.vk_invalidate_count),
	}
}

func (reactor *reactorDLL) runMain(payload [30]loadedUploads, timestep, output unsafe.Pointer, imageGeneration, contextGeneration, jointGeneration, timestepIdentity uint64, evaluationStart time.Time) (runMetrics, error) {
	var metrics runMetrics
	var evidence C.PrometheusModelBlockEvidence
	blockID := C.uint64_t(reactor.ownerID)
	var outputIdentity uint64
	var nextPrefetch <-chan prefetchResult
	var previousComputeStart time.Time
	var previousComputeEnd time.Time
	for layer := 0; layer < 30; layer++ {
		var status C.int
		var rebindDuration uint64
		if reactor.profile == C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH && layer > 0 {
			var activateErr error
			var prefetchEntry prefetchResult
			evidence, prefetchEntry, rebindDuration, activateErr = reactor.activatePrefetch(nextPrefetch)
			recordPrefetchOverlap(&metrics, prefetchEntry, previousComputeStart, previousComputeEnd)
			if activateErr != nil {
				return metrics, activateErr
			}
			status = C.PROM_OK
		} else {
			rebindStart := time.Now()
			status = C.oct_prom_owner_retarget(&reactor.api, reactor.runtime, C.uint64_t(reactor.sessionID), C.uint32_t(layer), payload[layer].pointer, C.uint32_t(len(payload[layer].buffers)), &evidence)
			rebindDuration = uint64(time.Since(rebindStart))
		}
		metrics.parameterRebindNS += rebindDuration
		metrics.stageRebindNS[4+layer] += rebindDuration
		metrics.uploadedWeightBytes += payload[layer].byteCount
		metrics.stageUploadedBytes[4+layer] += payload[layer].byteCount
		if status != C.PROM_OK {
			return metrics, fmt.Errorf("retarget MainTransformer%d: detail=%d", layer, int32(evidence.detail_code))
		}
		if layer+1 < len(payload) {
			nextPrefetch = reactor.prefetch(uint32(layer+1), payload[layer+1])
		}
		outputIdentity = identityFromText(fmt.Sprintf("main:%016x:%d", timestepIdentity, layer))
		previousComputeStart = time.Now()
		if C.oct_prom_main_execute(&reactor.api, reactor.runtime, blockID, C.uint64_t(reactor.sessionID), C.uint32_t(layer), C.uint64_t(imageGeneration), C.uint64_t(contextGeneration), C.uint64_t(jointGeneration), timestep, C.uint64_t(timestepBF16Bytes), C.uint64_t(timestepIdentity), C.uint64_t(outputIdentity), &evidence) != C.PROM_OK {
			return metrics, fmt.Errorf("execute MainTransformer%d: detail=%d", layer, int32(evidence.detail_code))
		}
		previousComputeEnd = time.Now()
		recordMainLayerTrace(&metrics.mainTrace[layer], &evidence, layer, evaluationStart, previousComputeStart, previousComputeEnd)
		metrics.modelExecutionNS += uint64(evidence.last_execution_ns)
		metrics.stageExecutionNS[4+layer] += uint64(evidence.last_execution_ns)
		metrics.stageGPUExecutionNS[4+layer] += uint64(evidence.gpu_compute_ns)
		metrics.mainLayerCount++
		jointGeneration++
	}
	if C.oct_prom_main_audit_final(&reactor.api, reactor.runtime, blockID, evidence.output_generation, C.uint64_t(outputIdentity), (*C.float)(output), C.uint64_t(uint64(jointTokens)*uint64(modelWidth)), &evidence) != C.PROM_OK {
		return metrics, fmt.Errorf("read back MainTransformer29 final boundary: detail=%d", int32(evidence.detail_code))
	}
	metrics.finalReadbackGPUNS = uint64(evidence.gpu_readback_ns)
	metrics.finalReadbackHostNS = uint64(evidence.last_output_readback_ns)
	metrics.persistentBytes = uint64(evidence.persistent_bytes)
	metrics.reusableBytes = uint64(evidence.reusable_bytes)
	metrics.auditBytes = uint64(evidence.audit_bytes)
	metrics.allocationCeilingBytes = metrics.persistentBytes + metrics.reusableBytes + metrics.auditBytes
	expectedCeiling := modelAllocationCeiling
	if reactor.profile == C.PROM_MODEL_EXECUTION_PROFILE_PREFETCH {
		expectedCeiling += 361820672
	}
	if metrics.allocationCeilingBytes != expectedCeiling {
		return metrics, fmt.Errorf("model-owned allocation ceiling mismatch: got %d want %d", metrics.allocationCeilingBytes, expectedCeiling)
	}
	return metrics, nil
}

func addMetrics(target *runMetrics, source runMetrics) {
	target.modelExecutionNS += source.modelExecutionNS
	target.parameterRebindNS += source.parameterRebindNS
	target.uploadedWeightBytes += source.uploadedWeightBytes
	target.prefetchTransferNS += source.prefetchTransferNS
	target.prefetchOverlapNS += source.prefetchOverlapNS
	target.prefetchWaitNS += source.prefetchWaitNS
	target.prefetchCount += source.prefetchCount
	if source.finalReadbackGPUNS != 0 || source.finalReadbackHostNS != 0 {
		target.finalReadbackGPUNS = source.finalReadbackGPUNS
		target.finalReadbackHostNS = source.finalReadbackHostNS
	}
	if source.allocationCeilingBytes != 0 {
		target.allocationCeilingBytes = source.allocationCeilingBytes
		target.persistentBytes = source.persistentBytes
		target.reusableBytes = source.reusableBytes
		target.auditBytes = source.auditBytes
	}
	target.mainLayerCount += source.mainLayerCount
	for index := range target.stageExecutionNS {
		target.stageExecutionNS[index] += source.stageExecutionNS[index]
		target.stageGPUExecutionNS[index] += source.stageGPUExecutionNS[index]
		target.stageRebindNS[index] += source.stageRebindNS[index]
		target.stagePayloadReadNS[index] += source.stagePayloadReadNS[index]
		target.stageUploadedBytes[index] += source.stageUploadedBytes[index]
	}
	for layer := range source.mainTrace {
		if source.mainTrace[layer].correlationID != 0 {
			target.mainTrace[layer] = source.mainTrace[layer]
		}
	}
}

func main() {}

//go:build windows && cgo

package main

/*
#include <stdint.h>
#include <stdlib.h>

typedef struct PrometheusZImageSessionCreateRequest {
  uint32_t struct_size;
  const char* reactor_dll_path;
  const char* compiled_model_lock_path;
  const char* payload_root;
  int32_t device_index;
} PrometheusZImageSessionCreateRequest;

typedef struct PrometheusZImageExecuteRequest {
  uint32_t struct_size;
  const void* image_bf16;
  uint64_t image_bytes;
  uint32_t image_batch;
  uint32_t image_tokens;
  uint32_t image_width;
  const float* context_fp32;
  uint64_t context_bytes;
  uint32_t context_batch;
  uint32_t context_tokens;
  uint32_t context_width;
  const void* timestep_bf16;
  uint64_t timestep_bytes;
  uint32_t timestep_batch;
  uint32_t timestep_width;
  float* output_image_fp32;
  uint64_t output_image_bytes;
} PrometheusZImageExecuteRequest;

typedef struct PrometheusZImageExecuteEvidence {
  uint32_t struct_size;
  uint32_t evaluation_index;
  uint32_t main_layer_count;
  uint32_t context_reused;
  uint64_t wall_time_ns;
  uint64_t model_execution_ns;
  uint64_t parameter_rebind_ns;
  uint64_t uploaded_weight_bytes;
  uint64_t model_allocation_ceiling_bytes;
  uint64_t persistent_bytes;
  uint64_t reusable_bytes;
  uint64_t audit_bytes;
  uint64_t host_package_cache_bytes;
  uint64_t host_package_cache_hits;
  uint64_t stage_execution_ns[34];
  uint64_t stage_rebind_ns[34];
  uint64_t stage_payload_read_ns[34];
  uint64_t stage_uploaded_weight_bytes[34];
} PrometheusZImageExecuteEvidence;

static void oct_prom_set_execute_stage(PrometheusZImageExecuteEvidence* evidence,
                                       uint32_t index, uint64_t execution_ns,
                                       uint64_t rebind_ns, uint64_t payload_read_ns,
                                       uint64_t uploaded_weight_bytes) {
  if (evidence == NULL || index >= 34u) return;
  evidence->stage_execution_ns[index] = execution_ns;
  evidence->stage_rebind_ns[index] = rebind_ns;
  evidence->stage_payload_read_ns[index] = payload_read_ns;
  evidence->stage_uploaded_weight_bytes[index] = uploaded_weight_bytes;
}
*/
import "C"

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sync"
	"time"
	"unsafe"
)

const outputImageFP32Bytes = uint64(imageTokens) * uint64(modelWidth) * 4

type bridgeSession struct {
	mu                sync.Mutex
	payload           validatedPayload
	hostPackages      *cachedPayload
	reactor           *reactorDLL
	contextDigest     [32]byte
	contextGeneration uint64
	evaluationIndex   uint32
	lastError         string
}

var bridgeSessions = struct {
	sync.Mutex
	next     uint64
	sessions map[uint64]*bridgeSession
	global   string
}{next: 1, sessions: make(map[uint64]*bridgeSession)}

func setGlobalError(err error) {
	bridgeSessions.Lock()
	defer bridgeSessions.Unlock()
	bridgeSessions.global = err.Error()
}

func sessionForHandle(handle uint64) (*bridgeSession, bool) {
	bridgeSessions.Lock()
	defer bridgeSessions.Unlock()
	session, ok := bridgeSessions.sessions[handle]
	return session, ok
}

func removeSession(handle uint64) *bridgeSession {
	bridgeSessions.Lock()
	defer bridgeSessions.Unlock()
	session := bridgeSessions.sessions[handle]
	delete(bridgeSessions.sessions, handle)
	return session
}

func copyErrorText(message string, destination *C.char, capacity C.uint64_t) C.uint64_t {
	required := uint64(len(message) + 1)
	if destination == nil || capacity == 0 {
		return C.uint64_t(required)
	}
	available := uint64(capacity)
	count := uint64(len(message))
	if count >= available {
		count = available - 1
	}
	buffer := unsafe.Slice((*byte)(unsafe.Pointer(destination)), int(available))
	copy(buffer[:count], message[:count])
	buffer[count] = 0
	return C.uint64_t(required)
}

func checkedCString(value *C.char, label string) (string, error) {
	if value == nil {
		return "", fmt.Errorf("%s is null", label)
	}
	text := C.GoString(value)
	if text == "" {
		return "", fmt.Errorf("%s is empty", label)
	}
	return text, nil
}

func bf16Finite(pointer unsafe.Pointer, byteCount uint64) bool {
	values := unsafe.Slice((*uint16)(pointer), int(byteCount/2))
	for _, value := range values {
		if value&0x7f80 == 0x7f80 {
			return false
		}
	}
	return true
}

func fp32Finite(pointer *C.float, byteCount uint64) bool {
	values := unsafe.Slice((*float32)(unsafe.Pointer(pointer)), int(byteCount/4))
	for _, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return false
		}
	}
	return true
}

func memoryIdentity(pointer unsafe.Pointer, byteCount uint64) uint64 {
	digest := sha256.Sum256(unsafe.Slice((*byte)(pointer), int(byteCount)))
	identity := binary.BigEndian.Uint64(digest[:8])
	if identity == 0 {
		return 1
	}
	return identity
}

func validateExecute(request *C.PrometheusZImageExecuteRequest, evidence *C.PrometheusZImageExecuteEvidence) error {
	if request == nil || evidence == nil {
		return fmt.Errorf("execute request and evidence are required")
	}
	if uint32(request.struct_size) != uint32(C.sizeof_PrometheusZImageExecuteRequest) {
		return fmt.Errorf("execute request struct_size=%d want %d", uint32(request.struct_size), uint32(C.sizeof_PrometheusZImageExecuteRequest))
	}
	if uint32(evidence.struct_size) != uint32(C.sizeof_PrometheusZImageExecuteEvidence) {
		return fmt.Errorf("execute evidence struct_size=%d want %d", uint32(evidence.struct_size), uint32(C.sizeof_PrometheusZImageExecuteEvidence))
	}
	if request.image_bf16 == nil || request.context_fp32 == nil || request.timestep_bf16 == nil || request.output_image_fp32 == nil {
		return fmt.Errorf("all execute tensor pointers are required")
	}
	if uint64(request.image_bytes) != imageBF16Bytes || uint32(request.image_batch) != 1 || uint32(request.image_tokens) != imageTokens || uint32(request.image_width) != modelWidth {
		return fmt.Errorf("image contract mismatch: require contiguous BF16 [1,%d,%d], %d bytes", imageTokens, modelWidth, imageBF16Bytes)
	}
	if uint64(request.context_bytes) != contextFP32Bytes || uint32(request.context_batch) != 1 || uint32(request.context_tokens) != contextTokens || uint32(request.context_width) != modelWidth {
		return fmt.Errorf("context contract mismatch: require contiguous FP32 [1,%d,%d], %d bytes", contextTokens, modelWidth, contextFP32Bytes)
	}
	if uint64(request.timestep_bytes) != timestepBF16Bytes || uint32(request.timestep_batch) != 1 || uint32(request.timestep_width) != timestepWidth {
		return fmt.Errorf("timestep contract mismatch: require contiguous BF16 [1,%d], %d bytes", timestepWidth, timestepBF16Bytes)
	}
	if uint64(request.output_image_bytes) != outputImageFP32Bytes {
		return fmt.Errorf("output contract mismatch: require contiguous FP32 [1,%d,%d], %d bytes", imageTokens, modelWidth, outputImageFP32Bytes)
	}
	if !bf16Finite(request.image_bf16, imageBF16Bytes) || !fp32Finite(request.context_fp32, contextFP32Bytes) || !bf16Finite(request.timestep_bf16, timestepBF16Bytes) {
		return fmt.Errorf("execute input contains NaN or infinity")
	}
	return nil
}

//export prometheus_zimage_bridge_abi_version
func prometheus_zimage_bridge_abi_version() C.uint32_t {
	return C.uint32_t(bridgeABIVersion)
}

//export prometheus_zimage_session_create
func prometheus_zimage_session_create(request *C.PrometheusZImageSessionCreateRequest, outHandle *C.uint64_t) C.int {
	if request == nil || outHandle == nil || uint32(request.struct_size) != uint32(C.sizeof_PrometheusZImageSessionCreateRequest) {
		setGlobalError(fmt.Errorf("invalid session create request"))
		return 1
	}
	*outHandle = 0
	if int32(request.device_index) != -1 && int32(request.device_index) != 0 {
		setGlobalError(fmt.Errorf("device_index=%d is unsupported; use -1 or 0", int32(request.device_index)))
		return 1
	}
	reactorPath, err := checkedCString(request.reactor_dll_path, "reactor_dll_path")
	if err != nil {
		setGlobalError(err)
		return 1
	}
	lockPath, err := checkedCString(request.compiled_model_lock_path, "compiled_model_lock_path")
	if err != nil {
		setGlobalError(err)
		return 1
	}
	payloadRoot, err := checkedCString(request.payload_root, "payload_root")
	if err != nil {
		setGlobalError(err)
		return 1
	}
	payload, err := validatePayloadRoot(lockPath, payloadRoot)
	if err != nil {
		setGlobalError(err)
		return 1
	}
	reactor, err := openReactor(reactorPath)
	if err != nil {
		setGlobalError(err)
		return 1
	}
	hostPackages, err := cachePayload(payload)
	if err != nil {
		_ = reactor.close()
		setGlobalError(err)
		return 1
	}
	session := &bridgeSession{payload: payload, hostPackages: hostPackages, reactor: reactor}
	bridgeSessions.Lock()
	handle := bridgeSessions.next
	bridgeSessions.next++
	bridgeSessions.sessions[handle] = session
	bridgeSessions.global = ""
	bridgeSessions.Unlock()
	*outHandle = C.uint64_t(handle)
	return 0
}

//export prometheus_zimage_session_execute
func prometheus_zimage_session_execute(handle C.uint64_t, request *C.PrometheusZImageExecuteRequest, evidence *C.PrometheusZImageExecuteEvidence) C.int {
	session, ok := sessionForHandle(uint64(handle))
	if !ok {
		setGlobalError(fmt.Errorf("unknown Prometheus Z-Image session %d", uint64(handle)))
		return 1
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := validateExecute(request, evidence); err != nil {
		session.lastError = err.Error()
		return 1
	}
	start := time.Now()
	metrics := runMetrics{}
	imageIdentity := memoryIdentity(request.image_bf16, imageBF16Bytes)
	timestepIdentity := memoryIdentity(request.timestep_bf16, timestepBF16Bytes)
	imageGeneration, imageMetrics, err := session.reactor.prepareImage(session.hostPackages.noise, request.image_bf16, request.timestep_bf16, imageIdentity, timestepIdentity)
	addMetrics(&metrics, imageMetrics)
	metrics.hostPackageCacheHits += 2
	if err != nil {
		session.lastError = err.Error()
		return 1
	}
	/* PreparedContext is evaluation-lifetime state: it depends on the current
	   timestep/modulation and is deliberately recomputed after PreparedImage. */
	contextGeneration, contextMetrics, err := session.reactor.prepareContext(session.hostPackages.context, unsafe.Pointer(request.context_fp32), memoryIdentity(unsafe.Pointer(request.context_fp32), contextFP32Bytes))
	addMetrics(&metrics, contextMetrics)
	if err != nil {
		session.lastError = err.Error()
		return 1
	}
	session.contextGeneration = contextGeneration
	metrics.hostPackageCacheHits += 2
	jointGeneration, err := session.reactor.compose(imageGeneration, contextGeneration)
	if err != nil {
		session.lastError = err.Error()
		return 1
	}
	joint := make([]float32, uint64(jointTokens)*uint64(modelWidth))
	mainMetrics, err := session.reactor.runMain(session.hostPackages.main, request.timestep_bf16, unsafe.Pointer(&joint[0]), imageGeneration, contextGeneration, jointGeneration, timestepIdentity)
	addMetrics(&metrics, mainMetrics)
	metrics.hostPackageCacheHits += 30
	if err != nil {
		session.lastError = err.Error()
		return 1
	}
	if !fp32Finite((*C.float)(unsafe.Pointer(&joint[0])), jointFP32Bytes) {
		session.lastError = "Prometheus returned NaN or infinity at the layer-29 boundary"
		return 1
	}
	output := unsafe.Slice((*float32)(unsafe.Pointer(request.output_image_fp32)), int(uint64(imageTokens)*uint64(modelWidth)))
	copy(output, joint[:len(output)])
	session.evaluationIndex++
	metrics.wallTimeNS = uint64(time.Since(start))
	evidence.evaluation_index = C.uint32_t(session.evaluationIndex)
	evidence.main_layer_count = C.uint32_t(metrics.mainLayerCount)
	evidence.context_reused = 0
	evidence.wall_time_ns = C.uint64_t(metrics.wallTimeNS)
	evidence.model_execution_ns = C.uint64_t(metrics.modelExecutionNS)
	evidence.parameter_rebind_ns = C.uint64_t(metrics.parameterRebindNS)
	evidence.uploaded_weight_bytes = C.uint64_t(metrics.uploadedWeightBytes)
	evidence.model_allocation_ceiling_bytes = C.uint64_t(metrics.allocationCeilingBytes)
	evidence.persistent_bytes = C.uint64_t(metrics.persistentBytes)
	evidence.reusable_bytes = C.uint64_t(metrics.reusableBytes)
	evidence.audit_bytes = C.uint64_t(metrics.auditBytes)
	evidence.host_package_cache_bytes = C.uint64_t(session.hostPackages.bytes)
	evidence.host_package_cache_hits = C.uint64_t(metrics.hostPackageCacheHits)
	for index := range metrics.stageExecutionNS {
		C.oct_prom_set_execute_stage(evidence, C.uint32_t(index), C.uint64_t(metrics.stageExecutionNS[index]), C.uint64_t(metrics.stageRebindNS[index]), C.uint64_t(metrics.stagePayloadReadNS[index]), C.uint64_t(metrics.stageUploadedBytes[index]))
	}
	session.lastError = ""
	return 0
}

//export prometheus_zimage_session_destroy
func prometheus_zimage_session_destroy(handle C.uint64_t) C.int {
	session := removeSession(uint64(handle))
	if session == nil {
		setGlobalError(fmt.Errorf("unknown Prometheus Z-Image session %d", uint64(handle)))
		return 1
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	err := session.reactor.close()
	session.hostPackages.free()
	if err != nil {
		setGlobalError(err)
		return 1
	}
	return 0
}

//export prometheus_zimage_last_error
func prometheus_zimage_last_error(handle C.uint64_t, destination *C.char, capacity C.uint64_t) C.uint64_t {
	if session, ok := sessionForHandle(uint64(handle)); ok {
		session.mu.Lock()
		defer session.mu.Unlock()
		return copyErrorText(session.lastError, destination, capacity)
	}
	bridgeSessions.Lock()
	defer bridgeSessions.Unlock()
	return copyErrorText(bridgeSessions.global, destination, capacity)
}

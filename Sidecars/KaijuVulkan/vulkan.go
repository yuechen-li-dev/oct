package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary/kaijuvulkan"
	vk "kaijuengine.com/rendering/vulkan"
	vc "kaijuengine.com/rendering/vulkan_const"
)

const (
	errorUnsupportedProtocol   = "unsupported_protocol"
	errorUnknownOperation      = "unknown_operation"
	errorMalformedSPIRV        = "malformed_spirv"
	errorHashMismatch          = "hash_mismatch"
	errorMissingEntryPoint     = "missing_entry_point"
	errorDuplicateBinding      = "duplicate_binding"
	errorUnsupportedSet        = "unsupported_descriptor_set"
	errorUnsupportedKind       = "unsupported_resource_kind"
	errorUnsupportedType       = "unsupported_element_type"
	errorByteLengthMismatch    = "byte_length_mismatch"
	errorOversizedRequest      = "oversized_request"
	errorInvalidDispatch       = "invalid_dispatch"
	errorInvalidPushConstants  = "invalid_push_constants"
	errorVulkanUnavailable     = "vulkan_unavailable"
	errorNoComputeDevice       = "no_compute_device"
	errorPipelineFailure       = "pipeline_creation_failure"
	errorDescriptorFailure     = "descriptor_failure"
	errorCommandFailure        = "command_failure"
	errorSyncFailure           = "synchronization_failure"
	errorTimestampFailure      = "timestamp_failure"
	errorReadbackFailure       = "readback_failure"
	errorDeviceLoss            = "device_loss"
	auditMemoryPlacementEnv    = "OCT_KAIJU_VULKAN_M38A_MEMORY"
	auditMemoryHostVisible     = "host-visible-coherent"
	auditMemoryHostDeviceLocal = "host-visible-device-local"
	auditReuploadInputsEnv     = "OCT_KAIJU_VULKAN_M38A_REUPLOAD_INPUTS"
)

type validatedExecutionRequest struct {
	benchmarkID    string
	replayID       string
	spirvSHA256    string
	entryPoint     string
	spv            []byte
	workgroupSize  kaijuvulkan.UInt3
	dispatchGroups kaijuvulkan.UInt3
	pushConstants  []byte
	resources      []kaijuvulkan.Resource
	warmup         uint32
	iterations     uint32
	measureTiming  bool
}

type validationLayerInfo struct {
	requested bool
	available bool
	enabled   bool
}

type runtimeInfo struct {
	Device kaijuvulkan.DeviceInfo
}

type buffer struct {
	request             kaijuvulkan.Resource
	handle              vk.Buffer
	memory              vk.DeviceMemory
	data                unsafe.Pointer
	memoryTypeIndex     uint32
	memoryPropertyFlags vk.MemoryPropertyFlags
	usageFlags          vk.BufferUsageFlags
	sharingMode         uint32
	memoryAlignment     uint32
	memoryOffset        uint32
}

type context struct {
	instance       vk.Instance
	physical       vk.PhysicalDevice
	device         vk.Device
	queue          vk.Queue
	queueFamily    uint32
	properties     vk.PhysicalDeviceProperties
	memoryProps    vk.PhysicalDeviceMemoryProperties
	descriptorSet  vk.DescriptorSet
	descriptorPool vk.DescriptorPool
	setLayout      vk.DescriptorSetLayout
	pipelineLayout vk.PipelineLayout
	shader         vk.ShaderModule
	pipeline       vk.Pipeline
	commandPool    vk.CommandPool
	commandBuffer  vk.CommandBuffer
	fence          vk.Fence
	queryPool      vk.QueryPool
	buffers        []buffer
	validation     validationLayerInfo
	diagnostics    []kaijuvulkan.Diagnostic
	deviceLost     bool
}

func sidecarLimits() kaijuvulkan.Limits {
	return kaijuvulkan.Limits{
		MaxSPIRVBytes:             8 << 20,
		MaxResources:              16,
		MaxBytesPerResource:       8 << 20,
		MaxAggregatePayloadBytes:  32 << 20,
		MaxAggregateReadbackBytes: 32 << 20,
		MaxPushConstantBytes:      256,
		MaxWarmup:                 256,
		MaxIterations:             512,
		MaxDispatchDimension:      1 << 20,
		MaxResponseBytes:          40 << 20,
		TimeoutMS:                 30000,
	}
}

func protocolFailure(code, replayID, message string) kaijuvulkan.DispatchResponse {
	return kaijuvulkan.DispatchResponse{
		Success:    false,
		ErrorCode:  code,
		ReplayID:   replayID,
		Device:     deviceRecord(),
		Timing:     kaijuvulkan.Timing{Source: kaijuvulkan.TimingSourceNone, SamplesNS: nil},
		Validation: kaijuvulkan.ValidationStatus{},
		Errors: []kaijuvulkan.Diagnostic{{
			Severity:  kaijuvulkan.DiagnosticSeverityError,
			Type:      kaijuvulkan.DiagnosticTypeInput,
			MessageID: code,
			Message:   message,
		}},
	}
}

func validateDispatchRequest(request kaijuvulkan.DispatchRequest) (validatedExecutionRequest, *kaijuvulkan.DispatchResponse) {
	validated, code, message := validateCommon(validatedExecutionRequest{
		benchmarkID:    request.BenchmarkID,
		replayID:       request.ReplayID,
		spirvSHA256:    strings.ToLower(request.SpirvSHA256),
		entryPoint:     request.EntryPoint,
		spv:            append([]byte(nil), request.Spirv...),
		workgroupSize:  request.WorkgroupSize,
		dispatchGroups: request.DispatchGroups,
		pushConstants:  append([]byte(nil), request.PushConstants...),
		resources:      cloneResources(request.Resources),
		warmup:         0,
		iterations:     1,
		measureTiming:  false,
	})
	if code != "" {
		response := protocolFailure(code, request.ReplayID, message)
		response.BenchmarkID = request.BenchmarkID
		response.SpirvSHA256 = strings.ToLower(request.SpirvSHA256)
		return validated, &response
	}
	return validated, nil
}

func validateBenchmarkRequest(request kaijuvulkan.BenchmarkRequest) (validatedExecutionRequest, *kaijuvulkan.DispatchResponse) {
	validated, code, message := validateCommon(validatedExecutionRequest{
		benchmarkID:    request.BenchmarkID,
		replayID:       request.ReplayID,
		spirvSHA256:    strings.ToLower(request.SpirvSHA256),
		entryPoint:     request.EntryPoint,
		spv:            append([]byte(nil), request.Spirv...),
		workgroupSize:  request.WorkgroupSize,
		dispatchGroups: request.DispatchGroups,
		pushConstants:  append([]byte(nil), request.PushConstants...),
		resources:      cloneResources(request.Resources),
		warmup:         request.Warmup,
		iterations:     request.Iterations,
		measureTiming:  true,
	})
	if code != "" {
		response := protocolFailure(code, request.ReplayID, message)
		response.BenchmarkID = request.BenchmarkID
		response.SpirvSHA256 = strings.ToLower(request.SpirvSHA256)
		return validated, &response
	}
	return validated, nil
}

func validateCommon(request validatedExecutionRequest) (validatedExecutionRequest, string, string) {
	limits := sidecarLimits()
	if len(request.spv) == 0 || len(request.spv) > int(limits.MaxSPIRVBytes) {
		return request, errorOversizedRequest, fmt.Sprintf("SPIR-V size %d exceeds limit %d", len(request.spv), limits.MaxSPIRVBytes)
	}
	if len(request.spv)%4 != 0 || len(request.spv) < 4 {
		return request, errorMalformedSPIRV, "SPIR-V length must be divisible by four"
	}
	if binary.LittleEndian.Uint32(request.spv[:4]) != 0x07230203 {
		return request, errorMalformedSPIRV, "SPIR-V magic number mismatch"
	}
	sum := sha256.Sum256(request.spv)
	actual := hex.EncodeToString(sum[:])
	if request.spirvSHA256 == "" || !strings.EqualFold(request.spirvSHA256, actual) {
		return request, errorHashMismatch, fmt.Sprintf("SPIR-V hash mismatch: request=%s actual=%s", request.spirvSHA256, actual)
	}
	request.spirvSHA256 = actual
	if strings.TrimSpace(request.entryPoint) == "" {
		return request, errorMissingEntryPoint, "entry point must be non-empty"
	}
	if request.workgroupSize.X == 0 || request.workgroupSize.Y == 0 || request.workgroupSize.Z == 0 {
		return request, errorInvalidDispatch, "workgroup size must be non-zero"
	}
	if request.dispatchGroups.X == 0 || request.dispatchGroups.Y == 0 || request.dispatchGroups.Z == 0 {
		return request, errorInvalidDispatch, "dispatch groups must be non-zero"
	}
	for _, value := range []uint32{request.workgroupSize.X, request.workgroupSize.Y, request.workgroupSize.Z, request.dispatchGroups.X, request.dispatchGroups.Y, request.dispatchGroups.Z} {
		if value > limits.MaxDispatchDimension {
			return request, errorInvalidDispatch, fmt.Sprintf("dispatch dimension %d exceeds limit %d", value, limits.MaxDispatchDimension)
		}
	}
	if len(request.pushConstants) > int(limits.MaxPushConstantBytes) {
		return request, errorInvalidPushConstants, fmt.Sprintf("push constants exceed limit %d", limits.MaxPushConstantBytes)
	}
	if len(request.pushConstants)%4 != 0 {
		return request, errorInvalidPushConstants, "push constant byte length must be divisible by four"
	}
	if request.warmup > limits.MaxWarmup {
		return request, errorOversizedRequest, fmt.Sprintf("warmup %d exceeds limit %d", request.warmup, limits.MaxWarmup)
	}
	if request.iterations == 0 || request.iterations > limits.MaxIterations {
		return request, errorOversizedRequest, fmt.Sprintf("iterations %d exceeds limit %d", request.iterations, limits.MaxIterations)
	}
	if len(request.resources) > int(limits.MaxResources) {
		return request, errorOversizedRequest, fmt.Sprintf("resource count %d exceeds limit %d", len(request.resources), limits.MaxResources)
	}
	var payloadBytes uint64
	var readbackBytes uint64
	seen := map[string]struct{}{}
	for _, resource := range request.resources {
		key := fmt.Sprintf("%d:%d", resource.Set, resource.Binding)
		if _, ok := seen[key]; ok {
			return request, errorDuplicateBinding, fmt.Sprintf("duplicate binding %s", key)
		}
		seen[key] = struct{}{}
		if resource.Set != 0 {
			return request, errorUnsupportedSet, fmt.Sprintf("descriptor set %d is unsupported", resource.Set)
		}
		if resource.Kind != "" && resource.Kind != kaijuvulkan.ResourceKindStorageBuffer {
			return request, errorUnsupportedKind, fmt.Sprintf("binding %d kind %q unsupported", resource.Binding, resource.Kind)
		}
		switch resource.ElementType {
		case kaijuvulkan.ElementTypeBytes, kaijuvulkan.ElementTypeU32, kaijuvulkan.ElementTypeI32, kaijuvulkan.ElementTypeF32, kaijuvulkan.ElementTypeFloat2, kaijuvulkan.ElementTypeFloat4:
		default:
			return request, errorUnsupportedType, fmt.Sprintf("binding %d element type %q unsupported", resource.Binding, resource.ElementType)
		}
		if resource.Access != kaijuvulkan.ResourceAccessReadonly && resource.Access != kaijuvulkan.ResourceAccessReadwrite {
			return request, errorUnsupportedKind, fmt.Sprintf("binding %d access %q unsupported", resource.Binding, resource.Access)
		}
		if resource.ByteLength == 0 {
			return request, errorByteLengthMismatch, fmt.Sprintf("binding %d byteLength must be positive", resource.Binding)
		}
		if resource.ByteLength > limits.MaxBytesPerResource {
			return request, errorOversizedRequest, fmt.Sprintf("binding %d exceeds per-resource limit %d", resource.Binding, limits.MaxBytesPerResource)
		}
		if len(resource.Payload) != int(resource.ByteLength) {
			return request, errorByteLengthMismatch, fmt.Sprintf("binding %d payload length %d does not match byteLength %d", resource.Binding, len(resource.Payload), resource.ByteLength)
		}
		if resource.Readback && resource.Access != kaijuvulkan.ResourceAccessReadwrite {
			return request, errorReadbackFailure, fmt.Sprintf("binding %d cannot request readback with %s access", resource.Binding, resource.Access)
		}
		payloadBytes += uint64(resource.ByteLength)
		if resource.Readback {
			readbackBytes += uint64(resource.ByteLength)
		}
	}
	if payloadBytes > uint64(limits.MaxAggregatePayloadBytes) {
		return request, errorOversizedRequest, fmt.Sprintf("aggregate payload %d exceeds limit %d", payloadBytes, limits.MaxAggregatePayloadBytes)
	}
	if readbackBytes > uint64(limits.MaxAggregateReadbackBytes) {
		return request, errorOversizedRequest, fmt.Sprintf("aggregate readback %d exceeds limit %d", readbackBytes, limits.MaxAggregateReadbackBytes)
	}
	return request, "", ""
}

func execute(request validatedExecutionRequest) kaijuvulkan.DispatchResponse {
	response := kaijuvulkan.DispatchResponse{
		Success:     false,
		ErrorCode:   "",
		BenchmarkID: request.benchmarkID,
		ReplayID:    request.replayID,
		SpirvSHA256: request.spirvSHA256,
		Device:      deviceRecord(),
		Timing:      kaijuvulkan.Timing{Source: kaijuvulkan.TimingSourceNone, SamplesNS: nil},
		Validation:  kaijuvulkan.ValidationStatus{},
	}
	ctx := &context{}
	if err := ctx.initialize(request); err != nil {
		code := mapInitializeError(err)
		response.ErrorCode = code
		response.Device = ctx.deviceInfo()
		response.Validation = ctx.validationStatus()
		response.Errors = append(response.Errors, ctx.diagnosticFor(code, err.Error()))
		return response
	}
	defer ctx.destroy()
	response.Device = ctx.deviceInfo()
	var samples []uint64
	for i := uint32(0); i < request.warmup+request.iterations; i++ {
		ns, err := ctx.runOnce(request)
		if err != nil {
			code := mapRuntimeError(err)
			response.ErrorCode = code
			response.Validation = ctx.validationStatus()
			response.Errors = append(response.Errors, ctx.diagnosticFor(code, err.Error()))
			return response
		}
		if i >= request.warmup && request.measureTiming {
			samples = append(samples, ns)
		}
	}
	readbacks, err := ctx.readbacks()
	if err != nil {
		response.ErrorCode = errorReadbackFailure
		response.Validation = ctx.validationStatus()
		response.Errors = append(response.Errors, ctx.diagnosticFor(errorReadbackFailure, err.Error()))
		return response
	}
	response.Success = true
	response.Timing = kaijuvulkan.Timing{
		Source: func() string {
			if request.measureTiming {
				return kaijuvulkan.TimingSourceVulkanQueryPoolGPU
			}
			return kaijuvulkan.TimingSourceNone
		}(),
		StageSpan: func() string {
			if request.measureTiming {
				return kaijuvulkan.TimingStageBoundsComputeDispatch
			}
			return ""
		}(),
		TimestampStartStage: kaijuvulkan.TimingStageTopOfPipe,
		TimestampEndStage:   kaijuvulkan.TimingStageBottomOfPipe,
		IntervalCommands: []string{
			kaijuvulkan.TimingCommandBindPipeline,
			kaijuvulkan.TimingCommandBindDescriptorSets,
			kaijuvulkan.TimingCommandPushConstants,
			kaijuvulkan.TimingCommandDispatch,
		},
		DispatchesPerSample:     1,
		QueryResetLocation:      "command_buffer_before_start_timestamp",
		FenceWaitLocation:       "host_after_queue_submit",
		ResultRetrievalLocation: "host_after_fence_wait",
		SamplesNS:               samples,
	}
	response.Readbacks = readbacks
	response.Validation = ctx.validationStatus()
	response.Warnings, response.Errors = ctx.partitionDiagnostics(response.Errors)
	if estimateResponseBytes(response) > sidecarLimits().MaxResponseBytes {
		return protocolFailure(errorOversizedRequest, request.replayID, "response size exceeded limit")
	}
	return response
}

func discoverRuntime(requestValidation bool) (runtimeInfo, error) {
	ctx := &context{validation: validationLayerInfo{requested: requestValidation}}
	if err := ctx.initializeRuntimeOnly(); err != nil {
		return runtimeInfo{}, err
	}
	defer ctx.destroy()
	return runtimeInfo{Device: ctx.deviceInfo()}, nil
}

func (c *context) initializeRuntimeOnly() error {
	if err := c.initializeLoaderAndInstance(); err != nil {
		return err
	}
	return c.selectDevice()
}

func (c *context) initialize(request validatedExecutionRequest) error {
	c.validation.requested = os.Getenv("OCT_KAIJU_VULKAN_VALIDATION") == "1"
	if err := c.initializeLoaderAndInstance(); err != nil {
		return err
	}
	if err := c.selectDevice(); err != nil {
		return err
	}
	priority := float32(1)
	queueInfo := vk.DeviceQueueCreateInfo{
		SType:            vc.StructureTypeDeviceQueueCreateInfo,
		QueueFamilyIndex: c.queueFamily,
		QueueCount:       1,
		PQueuePriorities: &priority,
	}
	deviceCreate := vk.DeviceCreateInfo{
		SType:                vc.StructureTypeDeviceCreateInfo,
		QueueCreateInfoCount: 1,
		PQueueCreateInfos:    &queueInfo,
	}
	if err := check("vkCreateDevice", vk.CreateDevice(c.physical, &deviceCreate, nil, &c.device)); err != nil {
		return err
	}
	vk.GetDeviceQueue(c.device, c.queueFamily, 0, &c.queue)
	for _, resource := range request.resources {
		if err := c.createBuffer(resource); err != nil {
			return fmt.Errorf("%s: %w", errorDescriptorFailure, err)
		}
	}
	if err := c.createDescriptors(); err != nil {
		return fmt.Errorf("%s: %w", errorDescriptorFailure, err)
	}
	if err := c.createPipeline(request.entryPoint, request.spv, len(request.pushConstants)); err != nil {
		return fmt.Errorf("%s: %w", errorPipelineFailure, err)
	}
	if err := c.createCommands(request.measureTiming); err != nil {
		return fmt.Errorf("%s: %w", errorCommandFailure, err)
	}
	return nil
}

func (c *context) initializeLoaderAndInstance() error {
	if err := vk.SetDefaultGetInstanceProcAddr(); err != nil {
		return fmt.Errorf("%s: %w", errorVulkanUnavailable, err)
	}
	if err := vk.Init(); err != nil {
		return fmt.Errorf("%s: %w", errorVulkanUnavailable, err)
	}
	validationInfo, err := queryValidationLayer()
	if err != nil {
		c.diagnostics = append(c.diagnostics, kaijuvulkan.Diagnostic{
			Severity:  kaijuvulkan.DiagnosticSeverityWarning,
			Type:      kaijuvulkan.DiagnosticTypeCapability,
			MessageID: "validation_probe_failed",
			Message:   err.Error(),
		})
	}
	c.validation.available = validationInfo.available
	c.validation.requested = c.validation.requested && validationInfo.available
	appName := append([]byte(sidecarName), 0)
	engineName := append([]byte("kaiju-raw-vulkan"), 0)
	app := vk.ApplicationInfo{
		SType:              vc.StructureTypeApplicationInfo,
		PApplicationName:   (*vk.Char)(unsafe.Pointer(&appName[0])),
		ApplicationVersion: vk.MakeVersion(0, 1, 0),
		PEngineName:        (*vk.Char)(unsafe.Pointer(&engineName[0])),
		EngineVersion:      1,
		ApiVersion:         vk.MakeVersion(1, 0, 0),
	}
	createInfo := vk.InstanceCreateInfo{SType: vc.StructureTypeInstanceCreateInfo, PApplicationInfo: &app}
	if c.validation.requested {
		createInfo.SetEnabledLayerNames([]string{"VK_LAYER_KHRONOS_validation\x00"})
	}
	if err := check("vkCreateInstance", vk.CreateInstance(&createInfo, nil, &c.instance)); err != nil {
		return err
	}
	if err := vk.InitInstance(c.instance); err != nil {
		return fmt.Errorf("%s: %w", errorVulkanUnavailable, err)
	}
	c.validation.enabled = c.validation.requested
	return nil
}

func queryValidationLayer() (validationLayerInfo, error) {
	var count uint32
	if result := vk.EnumerateInstanceLayerProperties(&count, nil); result != vc.Success {
		return validationLayerInfo{}, fmt.Errorf("vkEnumerateInstanceLayerProperties(count): %d", result)
	}
	if count == 0 {
		return validationLayerInfo{}, nil
	}
	layers := make([]vk.LayerProperties, count)
	if result := vk.EnumerateInstanceLayerProperties(&count, &layers[0]); result != vc.Success {
		return validationLayerInfo{}, fmt.Errorf("vkEnumerateInstanceLayerProperties: %d", result)
	}
	for _, layer := range layers {
		name := vk.ToString(layer.LayerName[:])
		if name == "VK_LAYER_KHRONOS_validation" {
			return validationLayerInfo{available: true}, nil
		}
	}
	return validationLayerInfo{}, nil
}

func (c *context) selectDevice() error {
	var count uint32
	if err := check("vkEnumeratePhysicalDevices(count)", vk.EnumeratePhysicalDevices(c.instance, &count, nil)); err != nil {
		return err
	}
	if count == 0 {
		return errors.New(errorNoComputeDevice)
	}
	devices := make([]vk.PhysicalDevice, count)
	if err := check("vkEnumeratePhysicalDevices", vk.EnumeratePhysicalDevices(c.instance, &count, &devices[0])); err != nil {
		return err
	}
	type candidate struct {
		device      vk.PhysicalDevice
		queueFamily uint32
		properties  vk.PhysicalDeviceProperties
		score       int
	}
	var candidates []candidate
	for _, device := range devices {
		var queueCount uint32
		vk.GetPhysicalDeviceQueueFamilyProperties(device, &queueCount, nil)
		if queueCount == 0 {
			continue
		}
		queues := make([]vk.QueueFamilyProperties, queueCount)
		vk.GetPhysicalDeviceQueueFamilyProperties(device, &queueCount, &queues[0])
		var properties vk.PhysicalDeviceProperties
		vk.GetPhysicalDeviceProperties(device, &properties)
		for idx, queue := range queues {
			if queue.QueueCount == 0 || queue.QueueFlags&vk.QueueFlags(vc.QueueComputeBit) == 0 {
				continue
			}
			score := 1
			if properties.DeviceType == vc.PhysicalDeviceTypeDiscreteGpu {
				score = 2
			}
			if queue.TimestampValidBits > 0 {
				score += 10
			}
			candidates = append(candidates, candidate{
				device: device, queueFamily: uint32(idx), properties: properties, score: score,
			})
			break
		}
	}
	if len(candidates) == 0 {
		return errors.New(errorNoComputeDevice)
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score > candidates[j].score
		}
		if candidates[i].properties.VendorID != candidates[j].properties.VendorID {
			return candidates[i].properties.VendorID < candidates[j].properties.VendorID
		}
		return candidates[i].properties.DeviceID < candidates[j].properties.DeviceID
	})
	best := candidates[0]
	c.physical = best.device
	c.queueFamily = best.queueFamily
	c.properties = best.properties
	vk.GetPhysicalDeviceMemoryProperties(c.physical, &c.memoryProps)
	return nil
}

func (c *context) createBuffer(resource kaijuvulkan.Resource) error {
	createInfo := vk.BufferCreateInfo{
		SType:       vc.StructureTypeBufferCreateInfo,
		Size:        vk.DeviceSize(resource.ByteLength),
		Usage:       vk.BufferUsageFlags(vc.BufferUsageStorageBufferBit),
		SharingMode: vc.SharingModeExclusive,
	}
	item := buffer{request: resource, usageFlags: createInfo.Usage, sharingMode: uint32(createInfo.SharingMode)}
	if err := check("vkCreateBuffer", vk.CreateBuffer(c.device, &createInfo, nil, &item.handle)); err != nil {
		return err
	}
	var memoryRequirements vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(c.device, item.handle, &memoryRequirements)
	flags := make([]vk.MemoryPropertyFlags, c.memoryProps.MemoryTypeCount)
	for i := range flags {
		flags[i] = c.memoryProps.MemoryTypes[i].PropertyFlags
	}
	index, ok := selectBufferMemoryType(memoryRequirements.MemoryTypeBits, flags, os.Getenv(auditMemoryPlacementEnv))
	if !ok {
		return fmt.Errorf("binding %d: no compatible host-visible coherent memory type", resource.Binding)
	}
	item.memoryTypeIndex = index
	item.memoryPropertyFlags = c.memoryProps.MemoryTypes[index].PropertyFlags
	item.memoryAlignment = uint32(memoryRequirements.Alignment)
	item.memoryOffset = 0
	allocateInfo := vk.MemoryAllocateInfo{
		SType:           vc.StructureTypeMemoryAllocateInfo,
		AllocationSize:  memoryRequirements.Size,
		MemoryTypeIndex: index,
	}
	if err := check("vkAllocateMemory", vk.AllocateMemory(c.device, &allocateInfo, nil, &item.memory)); err != nil {
		return err
	}
	if err := check("vkBindBufferMemory", vk.BindBufferMemory(c.device, item.handle, item.memory, 0)); err != nil {
		return err
	}
	if err := check("vkMapMemory", vk.MapMemory(c.device, item.memory, 0, vk.DeviceSize(resource.ByteLength), 0, &item.data)); err != nil {
		return err
	}
	copy(unsafe.Slice((*byte)(item.data), resource.ByteLength), resource.Payload)
	c.buffers = append(c.buffers, item)
	return nil
}

func (c *context) memoryType(bits uint32, flags vk.MemoryPropertyFlags) (uint32, bool) {
	for i := uint32(0); i < c.memoryProps.MemoryTypeCount; i++ {
		if bits&(1<<i) != 0 && c.memoryProps.MemoryTypes[i].PropertyFlags&flags == flags {
			return i, true
		}
	}
	return 0, false
}

func selectBufferMemoryType(bits uint32, available []vk.MemoryPropertyFlags, override string) (uint32, bool) {
	hostMemory := vk.MemoryPropertyFlags(vc.MemoryPropertyHostVisibleBit | vc.MemoryPropertyHostCoherentBit)
	deviceLocalHostMemory := hostMemory | vk.MemoryPropertyFlags(vc.MemoryPropertyDeviceLocalBit)
	find := func(required vk.MemoryPropertyFlags) (uint32, bool) {
		for i, flags := range available {
			if bits&(1<<uint32(i)) != 0 && flags&required == required {
				return uint32(i), true
			}
		}
		return 0, false
	}
	if override == auditMemoryHostVisible {
		return find(hostMemory)
	}
	if index, ok := find(deviceLocalHostMemory); ok {
		return index, true
	}
	if override == auditMemoryHostDeviceLocal {
		return 0, false
	}
	return find(hostMemory)
}

func (c *context) createDescriptors() error {
	if len(c.buffers) == 0 {
		return nil
	}
	sort.Slice(c.buffers, func(i, j int) bool { return c.buffers[i].request.Binding < c.buffers[j].request.Binding })
	bindings := make([]vk.DescriptorSetLayoutBinding, len(c.buffers))
	for i, item := range c.buffers {
		bindings[i] = vk.DescriptorSetLayoutBinding{
			Binding:         item.request.Binding,
			DescriptorType:  vc.DescriptorTypeStorageBuffer,
			DescriptorCount: 1,
			StageFlags:      vk.ShaderStageFlags(vc.ShaderStageComputeBit),
		}
	}
	layoutCreateInfo := vk.DescriptorSetLayoutCreateInfo{
		SType:        vc.StructureTypeDescriptorSetLayoutCreateInfo,
		BindingCount: uint32(len(bindings)),
		PBindings:    &bindings[0],
	}
	if err := check("vkCreateDescriptorSetLayout", vk.CreateDescriptorSetLayout(c.device, &layoutCreateInfo, nil, &c.setLayout)); err != nil {
		return err
	}
	poolSize := vk.DescriptorPoolSize{Type: vc.DescriptorTypeStorageBuffer, DescriptorCount: uint32(len(bindings))}
	poolCreateInfo := vk.DescriptorPoolCreateInfo{
		SType:         vc.StructureTypeDescriptorPoolCreateInfo,
		MaxSets:       1,
		PoolSizeCount: 1,
		PPoolSizes:    &poolSize,
	}
	if err := check("vkCreateDescriptorPool", vk.CreateDescriptorPool(c.device, &poolCreateInfo, nil, &c.descriptorPool)); err != nil {
		return err
	}
	allocateInfo := vk.DescriptorSetAllocateInfo{
		SType:              vc.StructureTypeDescriptorSetAllocateInfo,
		DescriptorPool:     c.descriptorPool,
		DescriptorSetCount: 1,
		PSetLayouts:        &c.setLayout,
	}
	if err := check("vkAllocateDescriptorSets", vk.AllocateDescriptorSets(c.device, &allocateInfo, &c.descriptorSet)); err != nil {
		return err
	}
	bufferInfos := make([]vk.DescriptorBufferInfo, len(c.buffers))
	writes := make([]vk.WriteDescriptorSet, len(c.buffers))
	for i, item := range c.buffers {
		bufferInfos[i] = vk.DescriptorBufferInfo{Buffer: item.handle, Range: vk.DeviceSize(item.request.ByteLength)}
		writes[i] = vk.WriteDescriptorSet{
			SType:           vc.StructureTypeWriteDescriptorSet,
			DstSet:          c.descriptorSet,
			DstBinding:      item.request.Binding,
			DescriptorCount: 1,
			DescriptorType:  vc.DescriptorTypeStorageBuffer,
			PBufferInfo:     &bufferInfos[i],
		}
	}
	var pinner runtime.Pinner
	pinner.Pin(&bufferInfos[0])
	defer pinner.Unpin()
	vk.UpdateDescriptorSets(c.device, uint32(len(writes)), &writes[0], 0, nil)
	runtime.KeepAlive(bufferInfos)
	runtime.KeepAlive(writes)
	return nil
}

func (c *context) createPipeline(entry string, spv []byte, pushBytes int) error {
	words := make([]uint32, len(spv)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(spv[i*4:])
	}
	shaderCreateInfo := vk.ShaderModuleCreateInfo{
		SType:    vc.StructureTypeShaderModuleCreateInfo,
		CodeSize: uint(len(spv)),
		PCode:    &words[0],
	}
	if err := check("vkCreateShaderModule", vk.CreateShaderModule(c.device, &shaderCreateInfo, nil, &c.shader)); err != nil {
		return err
	}
	layoutCreateInfo := vk.PipelineLayoutCreateInfo{SType: vc.StructureTypePipelineLayoutCreateInfo}
	if c.setLayout != nil {
		layoutCreateInfo.SetLayoutCount = 1
		layoutCreateInfo.PSetLayouts = &c.setLayout
	}
	var pushRange vk.PushConstantRange
	if pushBytes > 0 {
		pushRange = vk.PushConstantRange{
			StageFlags: vk.ShaderStageFlags(vc.ShaderStageComputeBit),
			Size:       uint32(pushBytes),
		}
		layoutCreateInfo.PushConstantRangeCount = 1
		layoutCreateInfo.PPushConstantRanges = &pushRange
	}
	if err := check("vkCreatePipelineLayout", vk.CreatePipelineLayout(c.device, &layoutCreateInfo, nil, &c.pipelineLayout)); err != nil {
		return err
	}
	name := append([]byte(entry), 0)
	stage := vk.PipelineShaderStageCreateInfo{
		SType:  vc.StructureTypePipelineShaderStageCreateInfo,
		Stage:  vc.ShaderStageComputeBit,
		Module: c.shader,
		PName:  (*vk.Char)(unsafe.Pointer(&name[0])),
	}
	createInfo := vk.ComputePipelineCreateInfo{
		SType:             vc.StructureTypeComputePipelineCreateInfo,
		Stage:             stage,
		Layout:            c.pipelineLayout,
		BasePipelineIndex: -1,
	}
	return check("vkCreateComputePipelines", vk.CreateComputePipelines(c.device, nil, 1, &createInfo, nil, &c.pipeline))
}

func (c *context) createCommands(enableTiming bool) error {
	commandPoolCreate := vk.CommandPoolCreateInfo{
		SType:            vc.StructureTypeCommandPoolCreateInfo,
		Flags:            vk.CommandPoolCreateFlags(vc.CommandPoolCreateResetCommandBufferBit),
		QueueFamilyIndex: c.queueFamily,
	}
	if err := check("vkCreateCommandPool", vk.CreateCommandPool(c.device, &commandPoolCreate, nil, &c.commandPool)); err != nil {
		return err
	}
	commandAllocate := vk.CommandBufferAllocateInfo{
		SType:              vc.StructureTypeCommandBufferAllocateInfo,
		CommandPool:        c.commandPool,
		Level:              vc.CommandBufferLevelPrimary,
		CommandBufferCount: 1,
	}
	if err := check("vkAllocateCommandBuffers", vk.AllocateCommandBuffers(c.device, &commandAllocate, &c.commandBuffer)); err != nil {
		return err
	}
	fenceCreate := vk.FenceCreateInfo{SType: vc.StructureTypeFenceCreateInfo}
	if err := check("vkCreateFence", vk.CreateFence(c.device, &fenceCreate, nil, &c.fence)); err != nil {
		return err
	}
	if enableTiming {
		queryCreate := vk.QueryPoolCreateInfo{
			SType:      vc.StructureTypeQueryPoolCreateInfo,
			QueryType:  vc.QueryTypeTimestamp,
			QueryCount: 2,
		}
		if err := check("vkCreateQueryPool", vk.CreateQueryPool(c.device, &queryCreate, nil, &c.queryPool)); err != nil {
			return err
		}
	}
	return nil
}

func (c *context) runOnce(request validatedExecutionRequest) (uint64, error) {
	if os.Getenv(auditReuploadInputsEnv) == "1" {
		for _, item := range c.buffers {
			if item.request.Access == kaijuvulkan.ResourceAccessReadonly {
				copy(unsafe.Slice((*byte)(item.data), item.request.ByteLength), item.request.Payload)
			}
		}
	}
	if err := check("vkResetFences", vk.ResetFences(c.device, 1, &c.fence)); err != nil {
		return 0, err
	}
	if err := check("vkResetCommandBuffer", vk.ResetCommandBuffer(c.commandBuffer, 0)); err != nil {
		return 0, err
	}
	beginInfo := vk.CommandBufferBeginInfo{
		SType: vc.StructureTypeCommandBufferBeginInfo,
		Flags: vk.CommandBufferUsageFlags(vc.CommandBufferUsageOneTimeSubmitBit),
	}
	if err := check("vkBeginCommandBuffer", vk.BeginCommandBuffer(c.commandBuffer, &beginInfo)); err != nil {
		return 0, err
	}
	if request.measureTiming && c.queryPool != nil {
		vk.CmdResetQueryPool(c.commandBuffer, c.queryPool, 0, 2)
		vk.CmdWriteTimestamp(c.commandBuffer, vc.PipelineStageTopOfPipeBit, c.queryPool, 0)
	}
	vk.CmdBindPipeline(c.commandBuffer, vc.PipelineBindPointCompute, c.pipeline)
	if c.descriptorSet != nil {
		vk.CmdBindDescriptorSets(c.commandBuffer, vc.PipelineBindPointCompute, c.pipelineLayout, 0, 1, &c.descriptorSet, 0, nil)
	}
	if len(request.pushConstants) > 0 {
		vk.CmdPushConstants(c.commandBuffer, c.pipelineLayout, vk.ShaderStageFlags(vc.ShaderStageComputeBit), 0, uint32(len(request.pushConstants)), unsafe.Pointer(&request.pushConstants[0]))
	}
	vk.CmdDispatch(c.commandBuffer, request.dispatchGroups.X, request.dispatchGroups.Y, request.dispatchGroups.Z)
	if request.measureTiming && c.queryPool != nil {
		vk.CmdWriteTimestamp(c.commandBuffer, vc.PipelineStageBottomOfPipeBit, c.queryPool, 1)
	}
	if err := check("vkEndCommandBuffer", vk.EndCommandBuffer(c.commandBuffer)); err != nil {
		return 0, err
	}
	submitInfo := vk.SubmitInfo{
		SType:              vc.StructureTypeSubmitInfo,
		CommandBufferCount: 1,
		PCommandBuffers:    &c.commandBuffer,
	}
	if result := vk.QueueSubmit(c.queue, 1, &submitInfo, c.fence); result != vc.Success {
		if result == vc.ErrorDeviceLost {
			c.deviceLost = true
		}
		return 0, fmt.Errorf("vkQueueSubmit failed with Vulkan result %d", result)
	}
	if result := vk.WaitForFences(c.device, 1, &c.fence, vc.True, uint64(sidecarLimits().TimeoutMS)*uint64(time.Millisecond)); result != vc.Success {
		if result == vc.ErrorDeviceLost {
			c.deviceLost = true
		}
		return 0, fmt.Errorf("vkWaitForFences failed with Vulkan result %d", result)
	}
	if !request.measureTiming || c.queryPool == nil {
		return 0, nil
	}
	values := [2]uint64{}
	flags := vk.QueryResultFlags(vc.QueryResult64Bit | vc.QueryResultWaitBit)
	if err := check("vkGetQueryPoolResults", vk.GetQueryPoolResults(c.device, c.queryPool, 0, 2, uint(unsafe.Sizeof(values)), unsafe.Pointer(&values[0]), vk.DeviceSize(unsafe.Sizeof(values[0])), flags)); err != nil {
		return 0, err
	}
	return timestampDelta(values[0], values[1], c.properties.Limits.TimestampPeriod, c.queueTimestampValidBits()), nil
}

func timestampDelta(start, end uint64, period float32, validBits uint32) uint64 {
	delta := end - start
	if validBits > 0 && validBits < 64 {
		mask := (uint64(1) << validBits) - 1
		delta = (end - start) & mask
	}
	return uint64(float64(delta) * float64(period))
}

func (c *context) queueTimestampValidBits() uint32 {
	var queueCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(c.physical, &queueCount, nil)
	if queueCount == 0 {
		return 0
	}
	queues := make([]vk.QueueFamilyProperties, queueCount)
	vk.GetPhysicalDeviceQueueFamilyProperties(c.physical, &queueCount, &queues[0])
	if int(c.queueFamily) >= len(queues) {
		return 0
	}
	return queues[c.queueFamily].TimestampValidBits
}

func (c *context) readbacks() ([]kaijuvulkan.Readback, error) {
	readbacks := []kaijuvulkan.Readback{}
	for _, item := range c.buffers {
		if !item.request.Readback {
			continue
		}
		if item.data == nil {
			return nil, fmt.Errorf("binding %d has no mapped data", item.request.Binding)
		}
		payload := append([]byte(nil), unsafe.Slice((*byte)(item.data), item.request.ByteLength)...)
		readbacks = append(readbacks, kaijuvulkan.Readback{
			Set:     item.request.Set,
			Binding: item.request.Binding,
			Payload: payload,
		})
	}
	return readbacks, nil
}

func (c *context) validationStatus() kaijuvulkan.ValidationStatus {
	warnings, errorsCount := 0, 0
	for _, diagnostic := range c.diagnostics {
		if diagnostic.Severity == kaijuvulkan.DiagnosticSeverityError {
			errorsCount++
			continue
		}
		if diagnostic.Severity == kaijuvulkan.DiagnosticSeverityWarning {
			warnings++
		}
	}
	return kaijuvulkan.ValidationStatus{
		Requested:  c.validation.requested,
		Available:  c.validation.available,
		Enabled:    c.validation.enabled,
		Warnings:   warnings,
		Errors:     errorsCount,
		DeviceLost: c.deviceLost,
	}
}

func (c *context) partitionDiagnostics(existingErrors []kaijuvulkan.Diagnostic) ([]kaijuvulkan.Diagnostic, []kaijuvulkan.Diagnostic) {
	warnings := []kaijuvulkan.Diagnostic{}
	errorsOut := append([]kaijuvulkan.Diagnostic(nil), existingErrors...)
	for _, diagnostic := range c.diagnostics {
		if diagnostic.Severity == kaijuvulkan.DiagnosticSeverityError {
			errorsOut = append(errorsOut, diagnostic)
			continue
		}
		warnings = append(warnings, diagnostic)
	}
	return warnings, errorsOut
}

func (c *context) diagnosticFor(code, message string) kaijuvulkan.Diagnostic {
	kind := kaijuvulkan.DiagnosticTypeRuntime
	if strings.HasPrefix(code, "unsupported_") || strings.Contains(code, "hash") || strings.Contains(code, "dispatch") {
		kind = kaijuvulkan.DiagnosticTypeInput
	}
	return kaijuvulkan.Diagnostic{
		Severity:  kaijuvulkan.DiagnosticSeverityError,
		Type:      kind,
		MessageID: code,
		Message:   message,
	}
}

func (c *context) destroy() {
	if c.device != nil {
		_ = vk.DeviceWaitIdle(c.device)
	}
	if c.queryPool != nil {
		vk.DestroyQueryPool(c.device, c.queryPool, nil)
	}
	if c.fence != nil {
		vk.DestroyFence(c.device, c.fence, nil)
	}
	if c.commandPool != nil {
		vk.DestroyCommandPool(c.device, c.commandPool, nil)
	}
	if c.pipeline != nil {
		vk.DestroyPipeline(c.device, c.pipeline, nil)
	}
	if c.shader != nil {
		vk.DestroyShaderModule(c.device, c.shader, nil)
	}
	if c.pipelineLayout != nil {
		vk.DestroyPipelineLayout(c.device, c.pipelineLayout, nil)
	}
	if c.descriptorPool != nil {
		vk.DestroyDescriptorPool(c.device, c.descriptorPool, nil)
	}
	if c.setLayout != nil {
		vk.DestroyDescriptorSetLayout(c.device, c.setLayout, nil)
	}
	for _, item := range c.buffers {
		if item.data != nil {
			vk.UnmapMemory(c.device, item.memory)
		}
		if item.handle != nil {
			vk.DestroyBuffer(c.device, item.handle, nil)
		}
		if item.memory != nil {
			vk.FreeMemory(c.device, item.memory, nil)
		}
	}
	if c.device != nil {
		vk.DestroyDevice(c.device, nil)
	}
	if c.instance != nil {
		vk.DestroyInstance(c.instance, nil)
	}
}

func (c *context) deviceInfo() kaijuvulkan.DeviceInfo {
	info := deviceRecord()
	if c.physical == nil {
		return info
	}
	info.DeviceName = vk.ToString(c.properties.DeviceName[:])
	info.VendorID = c.properties.VendorID
	info.DeviceID = c.properties.DeviceID
	info.DriverVersion = c.properties.DriverVersion
	info.VulkanAPIVersion = vk.Version(c.properties.ApiVersion).String()
	info.TimestampPeriodNS = float64(c.properties.Limits.TimestampPeriod)
	info.TimestampValidBits = c.queueTimestampValidBits()
	info.QueueFamilyIndex = c.queueFamily
	var queueCount uint32
	vk.GetPhysicalDeviceQueueFamilyProperties(c.physical, &queueCount, nil)
	if queueCount > 0 {
		queues := make([]vk.QueueFamilyProperties, queueCount)
		vk.GetPhysicalDeviceQueueFamilyProperties(c.physical, &queueCount, &queues[0])
		if int(c.queueFamily) < len(queues) {
			info.QueueFlags = uint32(queues[c.queueFamily].QueueFlags)
		}
	}
	if len(c.buffers) > 0 {
		info.BufferMemoryTypeIndex = c.buffers[0].memoryTypeIndex
		info.BufferMemoryPropertyFlags = uint32(c.buffers[0].memoryPropertyFlags)
		info.BufferUsageFlags = uint32(c.buffers[0].usageFlags)
		info.BufferSharingMode = "VK_SHARING_MODE_EXCLUSIVE"
		info.BufferMemoryAlignment = c.buffers[0].memoryAlignment
		info.BufferMemoryOffset = c.buffers[0].memoryOffset
	}
	return info
}

func deviceRecord() kaijuvulkan.DeviceInfo {
	return kaijuvulkan.DeviceInfo{
		RuntimeName:         sidecarName,
		RuntimeVersion:      sidecarVersion + "+" + runtime.Version(),
		KaijuUpstreamCommit: upstreamCommit,
		KaijuForkCommit:     forkCommit,
		Headless:            true,
	}
}

func mapInitializeError(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, errorNoComputeDevice):
		return errorNoComputeDevice
	case strings.Contains(text, errorPipelineFailure):
		return errorPipelineFailure
	case strings.Contains(text, errorDescriptorFailure):
		return errorDescriptorFailure
	case strings.Contains(text, errorCommandFailure):
		return errorCommandFailure
	case strings.Contains(text, "DeviceLost"):
		return errorDeviceLoss
	default:
		return errorVulkanUnavailable
	}
}

func mapRuntimeError(err error) string {
	text := err.Error()
	switch {
	case strings.Contains(text, "vkGetQueryPoolResults"):
		return errorTimestampFailure
	case strings.Contains(text, "vkWaitForFences"), strings.Contains(text, "vkQueueSubmit"):
		if strings.Contains(text, fmt.Sprint(int32(vc.ErrorDeviceLost))) {
			return errorDeviceLoss
		}
		return errorSyncFailure
	case strings.Contains(text, "vkResetCommandBuffer"), strings.Contains(text, "vkBeginCommandBuffer"), strings.Contains(text, "vkEndCommandBuffer"):
		return errorCommandFailure
	default:
		return errorCommandFailure
	}
}

func estimateResponseBytes(response kaijuvulkan.DispatchResponse) uint32 {
	total := 256 + len(response.ReplayID) + len(response.SpirvSHA256) + len(response.BenchmarkID)
	for _, readback := range response.Readbacks {
		total += len(readback.Payload) + 16
	}
	total += len(response.Timing.SamplesNS) * 8
	for _, diagnostic := range append(append([]kaijuvulkan.Diagnostic(nil), response.Warnings...), response.Errors...) {
		total += len(diagnostic.MessageID) + len(diagnostic.Message) + len(diagnostic.Type) + len(diagnostic.Severity)
	}
	return uint32(total)
}

func check(op string, result vc.Result) error {
	if result != vc.Success {
		return fmt.Errorf("%s failed with Vulkan result %d", op, result)
	}
	return nil
}

func cloneResources(values []kaijuvulkan.Resource) []kaijuvulkan.Resource {
	out := make([]kaijuvulkan.Resource, len(values))
	for i, value := range values {
		out[i] = value
		out[i].Payload = append([]byte(nil), value.Payload...)
	}
	return out
}

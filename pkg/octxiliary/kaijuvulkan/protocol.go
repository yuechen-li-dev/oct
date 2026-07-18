package kaijuvulkan

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/yuechen-li-dev/oct/pkg/octxiliary"
)

const (
	Family          = "kaiju-vulkan"
	ProtocolVersion = 2

	OperationCapabilities = "compute.capabilities"
	OperationDispatch     = "compute.dispatch"
	OperationBenchmark    = "compute.benchmark"

	recordCapabilities       = "VulkanCapabilities"
	recordUInt3              = "VulkanUInt3"
	recordResource           = "VulkanResource"
	recordResourceList       = "VulkanResourceList"
	recordSpecialization     = "VulkanSpecializationConstant"
	recordSpecializationList = "VulkanSpecializationConstantList"
	recordDispatchRequest    = "VulkanDispatchRequest"
	recordBenchmarkRequest   = "VulkanBenchmarkRequest"
	recordReadback           = "VulkanReadback"
	recordReadbackList       = "VulkanReadbackList"
	recordDeviceInfo         = "VulkanDeviceInfo"
	recordTiming             = "VulkanTiming"
	recordValidation         = "VulkanValidationStatus"
	recordDiagnostic         = "VulkanDiagnostic"
	recordDiagnosticList     = "VulkanDiagnosticList"
	recordIntList            = "VulkanIntList"
	recordStringList         = "VulkanStringList"
	recordLimits             = "VulkanLimits"
	recordResponse           = "VulkanDispatchResponse"
)

const (
	ResourceAccessReadonly  = "readonly"
	ResourceAccessReadwrite = "readwrite"

	ResourceKindStorageBuffer = "storage_buffer"

	ElementTypeBytes  = "bytes"
	ElementTypeU32    = "u32"
	ElementTypeI32    = "i32"
	ElementTypeF32    = "f32"
	ElementTypeFloat2 = "float2"
	ElementTypeFloat4 = "float4"

	TimingSourceNone                 = "none"
	TimingSourceVulkanQueryPoolGPU   = "vulkan_query_pool_gpu_timestamp"
	TimingStageBoundsComputeDispatch = "top_of_pipe_to_bottom_of_pipe"
	TimingStageTopOfPipe             = "VK_PIPELINE_STAGE_TOP_OF_PIPE_BIT"
	TimingStageBottomOfPipe          = "VK_PIPELINE_STAGE_BOTTOM_OF_PIPE_BIT"
	TimingCommandBindPipeline        = "vkCmdBindPipeline"
	TimingCommandBindDescriptorSets  = "vkCmdBindDescriptorSets"
	TimingCommandPushConstants       = "vkCmdPushConstants"
	TimingCommandDispatch            = "vkCmdDispatch"
	DiagnosticSeverityInfo           = "info"
	DiagnosticSeverityWarning        = "warning"
	DiagnosticSeverityError          = "error"
	DiagnosticTypeValidation         = "validation"
	DiagnosticTypeRuntime            = "runtime"
	DiagnosticTypeCapability         = "capability"
	DiagnosticTypeInput              = "input"
)

type UInt3 struct {
	X uint32
	Y uint32
	Z uint32
}

type Resource struct {
	Set         uint32
	Binding     uint32
	Access      string
	Kind        string
	ElementType string
	ByteLength  uint32
	Payload     []byte
	Readback    bool
}

// SpecializationConstant is an explicitly typed uint32 pipeline specialization
// value.  It covers the public GGML compute shaders' local-size constants
// without making the Kaiju protocol a shader-language-specific compiler API.
type SpecializationConstant struct {
	ID    uint32
	Value uint32
}

type DispatchRequest struct {
	BenchmarkID             string
	ReplayID                string
	Spirv                   []byte
	SpirvSHA256             string
	EntryPoint              string
	WorkgroupSize           UInt3
	DispatchGroups          UInt3
	PushConstants           []byte
	SpecializationConstants []SpecializationConstant
	Resources               []Resource
}

type BenchmarkRequest struct {
	DispatchRequest
	Warmup     uint32
	Iterations uint32
}

type Readback struct {
	Set     uint32
	Binding uint32
	Payload []byte
}

type DeviceInfo struct {
	RuntimeName               string
	RuntimeVersion            string
	KaijuUpstreamCommit       string
	KaijuForkCommit           string
	DeviceName                string
	VendorID                  uint32
	DeviceID                  uint32
	DriverVersion             uint32
	VulkanAPIVersion          string
	TimestampPeriodNS         float64
	TimestampValidBits        uint32
	QueueFamilyIndex          uint32
	QueueFlags                uint32
	BufferMemoryTypeIndex     uint32
	BufferMemoryPropertyFlags uint32
	BufferUsageFlags          uint32
	BufferSharingMode         string
	BufferMemoryAlignment     uint32
	BufferMemoryOffset        uint32
	Headless                  bool
}

type Timing struct {
	Source                  string
	StageSpan               string
	TimestampStartStage     string
	TimestampEndStage       string
	IntervalCommands        []string
	DispatchesPerSample     uint32
	QueryResetLocation      string
	FenceWaitLocation       string
	ResultRetrievalLocation string
	SamplesNS               []uint64
}

type ValidationStatus struct {
	Requested  bool
	Available  bool
	Enabled    bool
	Warnings   int
	Errors     int
	DeviceLost bool
}

type Diagnostic struct {
	Severity  string
	Type      string
	MessageID string
	Message   string
}

type Limits struct {
	MaxSPIRVBytes             uint32
	MaxResources              uint32
	MaxBytesPerResource       uint32
	MaxAggregatePayloadBytes  uint32
	MaxAggregateReadbackBytes uint32
	MaxPushConstantBytes      uint32
	MaxWarmup                 uint32
	MaxIterations             uint32
	MaxDispatchDimension      uint32
	MaxResponseBytes          uint32
	TimeoutMS                 uint32
}

type Capabilities struct {
	Protocol            int
	KaijuUpstreamCommit string
	KaijuForkCommit     string
	OS                  string
	Architecture        string
	Headless            bool
	DispatchSupported   bool
	BenchmarkSupported  bool
	Set0Only            bool
	StorageBuffers      bool
	ExplicitEntryPoints bool
	PushConstants       bool
	QueryTimestamps     bool
	ValidationAvailable bool
	ValidationEnabled   bool
	WindowsProven       bool
	LinuxStatus         string
	SupportedAccesses   []string
	SupportedKinds      []string
	SupportedElements   []string
	Limits              Limits
	Device              DeviceInfo
}

type DispatchResponse struct {
	Success     bool
	ErrorCode   string
	BenchmarkID string
	ReplayID    string
	SpirvSHA256 string
	Device      DeviceInfo
	Timing      Timing
	Validation  ValidationStatus
	Readbacks   []Readback
	Warnings    []Diagnostic
	Errors      []Diagnostic
}

func DispatchRequestValue(v DispatchRequest) octxiliary.Value {
	return octxiliary.RecordValue(recordDispatchRequest, append(dispatchRequestFields(v), field("BenchmarkID", octxiliary.StringValue(v.BenchmarkID))))
}

func BenchmarkRequestValue(v BenchmarkRequest) octxiliary.Value {
	fields := dispatchRequestFields(v.DispatchRequest)
	fields = append(fields,
		field("BenchmarkID", octxiliary.StringValue(v.BenchmarkID)),
		field("Warmup", octxiliary.IntValue(int(v.Warmup))),
		field("Iterations", octxiliary.IntValue(int(v.Iterations))),
	)
	return octxiliary.RecordValue(recordBenchmarkRequest, fields)
}

func ParseDispatchRequestArg(req octxiliary.Request, index int) (DispatchRequest, error) {
	fields, err := octxiliary.ArgRecord(req, index, recordDispatchRequest)
	if err != nil {
		return DispatchRequest{}, err
	}
	return parseDispatchRequest(fields)
}

func ParseBenchmarkRequestArg(req octxiliary.Request, index int) (BenchmarkRequest, error) {
	fields, err := octxiliary.ArgRecord(req, index, recordBenchmarkRequest)
	if err != nil {
		return BenchmarkRequest{}, err
	}
	base, err := parseDispatchRequest(fields)
	if err != nil {
		return BenchmarkRequest{}, err
	}
	indexed := fieldsByName(fields)
	warmup, err := uint32Field(indexed, "Warmup")
	if err != nil {
		return BenchmarkRequest{}, err
	}
	iterations, err := uint32Field(indexed, "Iterations")
	if err != nil {
		return BenchmarkRequest{}, err
	}
	return BenchmarkRequest{DispatchRequest: base, Warmup: warmup, Iterations: iterations}, nil
}

func CapabilitiesValue(v Capabilities) octxiliary.Value {
	return octxiliary.RecordValue(recordCapabilities, []octxiliary.FieldValue{
		field("Protocol", octxiliary.IntValue(v.Protocol)),
		field("KaijuUpstreamCommit", octxiliary.StringValue(v.KaijuUpstreamCommit)),
		field("KaijuForkCommit", octxiliary.StringValue(v.KaijuForkCommit)),
		field("OS", octxiliary.StringValue(v.OS)),
		field("Architecture", octxiliary.StringValue(v.Architecture)),
		field("Headless", octxiliary.BoolValue(v.Headless)),
		field("DispatchSupported", octxiliary.BoolValue(v.DispatchSupported)),
		field("BenchmarkSupported", octxiliary.BoolValue(v.BenchmarkSupported)),
		field("Set0Only", octxiliary.BoolValue(v.Set0Only)),
		field("StorageBuffers", octxiliary.BoolValue(v.StorageBuffers)),
		field("ExplicitEntryPoints", octxiliary.BoolValue(v.ExplicitEntryPoints)),
		field("PushConstants", octxiliary.BoolValue(v.PushConstants)),
		field("QueryTimestamps", octxiliary.BoolValue(v.QueryTimestamps)),
		field("ValidationAvailable", octxiliary.BoolValue(v.ValidationAvailable)),
		field("ValidationEnabled", octxiliary.BoolValue(v.ValidationEnabled)),
		field("WindowsProven", octxiliary.BoolValue(v.WindowsProven)),
		field("LinuxStatus", octxiliary.StringValue(v.LinuxStatus)),
		field("SupportedAccesses", stringListValue(v.SupportedAccesses)),
		field("SupportedKinds", stringListValue(v.SupportedKinds)),
		field("SupportedElements", stringListValue(v.SupportedElements)),
		field("Limits", limitsValue(v.Limits)),
		field("Device", deviceInfoValue(v.Device)),
	})
}

func ParseCapabilitiesValue(value octxiliary.Value) (Capabilities, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordCapabilities {
		return Capabilities{}, fmt.Errorf("expected %s record", recordCapabilities)
	}
	indexed := fieldsByName(value.Fields)
	limitsValue, err := recordField(indexed, "Limits", recordLimits)
	if err != nil {
		return Capabilities{}, err
	}
	deviceValue, err := recordField(indexed, "Device", recordDeviceInfo)
	if err != nil {
		return Capabilities{}, err
	}
	limits, err := parseLimits(limitsValue.Fields)
	if err != nil {
		return Capabilities{}, err
	}
	device, err := parseDeviceInfo(deviceValue.Fields)
	if err != nil {
		return Capabilities{}, err
	}
	protocol, err := intField(indexed, "Protocol")
	if err != nil {
		return Capabilities{}, err
	}
	accesses, err := parseStringListValue(indexed["SupportedAccesses"])
	if err != nil {
		return Capabilities{}, err
	}
	kinds, err := parseStringListValue(indexed["SupportedKinds"])
	if err != nil {
		return Capabilities{}, err
	}
	elements, err := parseStringListValue(indexed["SupportedElements"])
	if err != nil {
		return Capabilities{}, err
	}
	return Capabilities{
		Protocol: protocol, KaijuUpstreamCommit: stringFieldMust(indexed, "KaijuUpstreamCommit"),
		KaijuForkCommit: stringFieldMust(indexed, "KaijuForkCommit"), OS: stringFieldMust(indexed, "OS"),
		Architecture: stringFieldMust(indexed, "Architecture"), Headless: boolFieldMust(indexed, "Headless"),
		DispatchSupported: boolFieldMust(indexed, "DispatchSupported"), BenchmarkSupported: boolFieldMust(indexed, "BenchmarkSupported"),
		Set0Only: boolFieldMust(indexed, "Set0Only"), StorageBuffers: boolFieldMust(indexed, "StorageBuffers"),
		ExplicitEntryPoints: boolFieldMust(indexed, "ExplicitEntryPoints"), PushConstants: boolFieldMust(indexed, "PushConstants"),
		QueryTimestamps: boolFieldMust(indexed, "QueryTimestamps"), ValidationAvailable: boolFieldMust(indexed, "ValidationAvailable"),
		ValidationEnabled: boolFieldMust(indexed, "ValidationEnabled"), WindowsProven: boolFieldMust(indexed, "WindowsProven"),
		LinuxStatus: stringFieldMust(indexed, "LinuxStatus"), SupportedAccesses: accesses, SupportedKinds: kinds,
		SupportedElements: elements, Limits: limits, Device: device,
	}, nil
}

func DispatchResponseValue(v DispatchResponse) octxiliary.Value {
	return octxiliary.RecordValue(recordResponse, []octxiliary.FieldValue{
		field("Success", octxiliary.BoolValue(v.Success)),
		field("ErrorCode", octxiliary.StringValue(v.ErrorCode)),
		field("BenchmarkID", octxiliary.StringValue(v.BenchmarkID)),
		field("ReplayID", octxiliary.StringValue(v.ReplayID)),
		field("SpirvSHA256", octxiliary.StringValue(v.SpirvSHA256)),
		field("Device", deviceInfoValue(v.Device)),
		field("Timing", timingValue(v.Timing)),
		field("Validation", validationValue(v.Validation)),
		field("Readbacks", readbackListValue(v.Readbacks)),
		field("Warnings", diagnosticListValue(v.Warnings)),
		field("Errors", diagnosticListValue(v.Errors)),
	})
}

func ParseDispatchResponseValue(value octxiliary.Value) (DispatchResponse, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordResponse {
		return DispatchResponse{}, fmt.Errorf("expected %s record", recordResponse)
	}
	indexed := fieldsByName(value.Fields)
	deviceValue, err := recordField(indexed, "Device", recordDeviceInfo)
	if err != nil {
		return DispatchResponse{}, err
	}
	timingRecord, err := recordField(indexed, "Timing", recordTiming)
	if err != nil {
		return DispatchResponse{}, err
	}
	validationRecord, err := recordField(indexed, "Validation", recordValidation)
	if err != nil {
		return DispatchResponse{}, err
	}
	readbacks, err := parseReadbackListValue(indexed["Readbacks"])
	if err != nil {
		return DispatchResponse{}, err
	}
	warnings, err := parseDiagnosticListValue(indexed["Warnings"])
	if err != nil {
		return DispatchResponse{}, err
	}
	errorsList, err := parseDiagnosticListValue(indexed["Errors"])
	if err != nil {
		return DispatchResponse{}, err
	}
	device, err := parseDeviceInfo(deviceValue.Fields)
	if err != nil {
		return DispatchResponse{}, err
	}
	timing, err := parseTiming(timingRecord.Fields)
	if err != nil {
		return DispatchResponse{}, err
	}
	validation, err := parseValidation(validationRecord.Fields)
	if err != nil {
		return DispatchResponse{}, err
	}
	return DispatchResponse{
		Success:     boolFieldMust(indexed, "Success"),
		ErrorCode:   stringFieldMust(indexed, "ErrorCode"),
		BenchmarkID: stringFieldMust(indexed, "BenchmarkID"),
		ReplayID:    stringFieldMust(indexed, "ReplayID"),
		SpirvSHA256: stringFieldMust(indexed, "SpirvSHA256"),
		Device:      device,
		Timing:      timing,
		Validation:  validation,
		Readbacks:   readbacks,
		Warnings:    warnings,
		Errors:      errorsList,
	}, nil
}

func dispatchRequestFields(v DispatchRequest) []octxiliary.FieldValue {
	return []octxiliary.FieldValue{
		field("ReplayID", octxiliary.StringValue(v.ReplayID)),
		field("Spirv", octxiliary.BytesValue(v.Spirv)),
		field("SpirvSHA256", octxiliary.StringValue(v.SpirvSHA256)),
		field("EntryPoint", octxiliary.StringValue(v.EntryPoint)),
		field("WorkgroupSize", uint3Value(v.WorkgroupSize)),
		field("DispatchGroups", uint3Value(v.DispatchGroups)),
		field("PushConstants", octxiliary.BytesValue(v.PushConstants)),
		field("SpecializationConstants", specializationListValue(v.SpecializationConstants)),
		field("Resources", resourceListValue(v.Resources)),
	}
}

func parseDispatchRequest(fields []octxiliary.FieldValue) (DispatchRequest, error) {
	indexed := fieldsByName(fields)
	workgroupValue, err := recordField(indexed, "WorkgroupSize", recordUInt3)
	if err != nil {
		return DispatchRequest{}, err
	}
	dispatchValue, err := recordField(indexed, "DispatchGroups", recordUInt3)
	if err != nil {
		return DispatchRequest{}, err
	}
	resources, err := parseResourceListValue(indexed["Resources"])
	if err != nil {
		return DispatchRequest{}, err
	}
	workgroup, err := parseUInt3(workgroupValue.Fields)
	if err != nil {
		return DispatchRequest{}, err
	}
	dispatchGroups, err := parseUInt3(dispatchValue.Fields)
	if err != nil {
		return DispatchRequest{}, err
	}
	spirv, err := bytesField(indexed, "Spirv")
	if err != nil {
		return DispatchRequest{}, err
	}
	push, err := bytesField(indexed, "PushConstants")
	if err != nil {
		return DispatchRequest{}, err
	}
	specializations := []SpecializationConstant{}
	if value, ok := indexed["SpecializationConstants"]; ok {
		specializations, err = parseSpecializationListValue(value)
		if err != nil {
			return DispatchRequest{}, err
		}
	}
	return DispatchRequest{
		BenchmarkID:             stringFieldMust(indexed, "BenchmarkID"),
		ReplayID:                stringFieldMust(indexed, "ReplayID"),
		Spirv:                   spirv,
		SpirvSHA256:             stringFieldMust(indexed, "SpirvSHA256"),
		EntryPoint:              stringFieldMust(indexed, "EntryPoint"),
		WorkgroupSize:           workgroup,
		DispatchGroups:          dispatchGroups,
		PushConstants:           push,
		SpecializationConstants: specializations,
		Resources:               resources,
	}, nil
}

func specializationListValue(values []SpecializationConstant) octxiliary.Value {
	items := append([]SpecializationConstant(nil), values...)
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(items)))}
	for i, value := range items {
		fields = append(fields, field("Item"+strconv.Itoa(i), octxiliary.RecordValue(recordSpecialization, []octxiliary.FieldValue{
			field("ID", octxiliary.IntValue(int(value.ID))),
			field("Value", octxiliary.IntValue(int(value.Value))),
		})))
	}
	return octxiliary.RecordValue(recordSpecializationList, fields)
}

func parseSpecializationListValue(value octxiliary.Value) ([]SpecializationConstant, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordSpecializationList {
		return nil, fmt.Errorf("expected %s record", recordSpecializationList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	items := make([]SpecializationConstant, 0, count)
	for i := 0; i < count; i++ {
		item, err := recordField(indexed, "Item"+strconv.Itoa(i), recordSpecialization)
		if err != nil {
			return nil, err
		}
		fields := fieldsByName(item.Fields)
		id, err := uint32Field(fields, "ID")
		if err != nil {
			return nil, err
		}
		constant, err := uint32Field(fields, "Value")
		if err != nil {
			return nil, err
		}
		items = append(items, SpecializationConstant{ID: id, Value: constant})
	}
	return items, nil
}

func uint3Value(v UInt3) octxiliary.Value {
	return octxiliary.RecordValue(recordUInt3, []octxiliary.FieldValue{
		field("X", octxiliary.IntValue(int(v.X))),
		field("Y", octxiliary.IntValue(int(v.Y))),
		field("Z", octxiliary.IntValue(int(v.Z))),
	})
}

func parseUInt3(fields []octxiliary.FieldValue) (UInt3, error) {
	indexed := fieldsByName(fields)
	x, err := uint32Field(indexed, "X")
	if err != nil {
		return UInt3{}, err
	}
	y, err := uint32Field(indexed, "Y")
	if err != nil {
		return UInt3{}, err
	}
	z, err := uint32Field(indexed, "Z")
	if err != nil {
		return UInt3{}, err
	}
	return UInt3{X: x, Y: y, Z: z}, nil
}

func resourceListValue(values []Resource) octxiliary.Value {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Set != values[j].Set {
			return values[i].Set < values[j].Set
		}
		return values[i].Binding < values[j].Binding
	})
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(values)))}
	for i, value := range values {
		fields = append(fields, field("Item"+strconv.Itoa(i), resourceValue(value)))
	}
	return octxiliary.RecordValue(recordResourceList, fields)
}

func parseResourceListValue(value octxiliary.Value) ([]Resource, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordResourceList {
		return nil, fmt.Errorf("expected %s record", recordResourceList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	out := make([]Resource, 0, count)
	for i := 0; i < count; i++ {
		item, err := recordField(indexed, "Item"+strconv.Itoa(i), recordResource)
		if err != nil {
			return nil, err
		}
		resource, err := parseResource(item.Fields)
		if err != nil {
			return nil, err
		}
		out = append(out, resource)
	}
	return out, nil
}

func resourceValue(v Resource) octxiliary.Value {
	return octxiliary.RecordValue(recordResource, []octxiliary.FieldValue{
		field("Set", octxiliary.IntValue(int(v.Set))),
		field("Binding", octxiliary.IntValue(int(v.Binding))),
		field("Access", octxiliary.StringValue(v.Access)),
		field("Kind", octxiliary.StringValue(v.Kind)),
		field("ElementType", octxiliary.StringValue(v.ElementType)),
		field("ByteLength", octxiliary.IntValue(int(v.ByteLength))),
		field("Payload", octxiliary.BytesValue(v.Payload)),
		field("Readback", octxiliary.BoolValue(v.Readback)),
	})
}

func parseResource(fields []octxiliary.FieldValue) (Resource, error) {
	indexed := fieldsByName(fields)
	setValue, err := uint32Field(indexed, "Set")
	if err != nil {
		return Resource{}, err
	}
	bindingValue, err := uint32Field(indexed, "Binding")
	if err != nil {
		return Resource{}, err
	}
	byteLength, err := uint32Field(indexed, "ByteLength")
	if err != nil {
		return Resource{}, err
	}
	payload, err := bytesField(indexed, "Payload")
	if err != nil {
		return Resource{}, err
	}
	readback, err := boolField(indexed, "Readback")
	if err != nil {
		return Resource{}, err
	}
	return Resource{
		Set: setValue, Binding: bindingValue, Access: stringFieldMust(indexed, "Access"),
		Kind: stringFieldMust(indexed, "Kind"), ElementType: stringFieldMust(indexed, "ElementType"),
		ByteLength: byteLength, Payload: payload, Readback: readback,
	}, nil
}

func readbackListValue(values []Readback) octxiliary.Value {
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(values)))}
	for i, value := range values {
		fields = append(fields, field("Item"+strconv.Itoa(i), octxiliary.RecordValue(recordReadback, []octxiliary.FieldValue{
			field("Set", octxiliary.IntValue(int(value.Set))),
			field("Binding", octxiliary.IntValue(int(value.Binding))),
			field("Payload", octxiliary.BytesValue(value.Payload)),
		})))
	}
	return octxiliary.RecordValue(recordReadbackList, fields)
}

func parseReadbackListValue(value octxiliary.Value) ([]Readback, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordReadbackList {
		return nil, fmt.Errorf("expected %s record", recordReadbackList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	out := make([]Readback, 0, count)
	for i := 0; i < count; i++ {
		item, err := recordField(indexed, "Item"+strconv.Itoa(i), recordReadback)
		if err != nil {
			return nil, err
		}
		fields := fieldsByName(item.Fields)
		setValue, err := uint32Field(fields, "Set")
		if err != nil {
			return nil, err
		}
		bindingValue, err := uint32Field(fields, "Binding")
		if err != nil {
			return nil, err
		}
		payload, err := bytesField(fields, "Payload")
		if err != nil {
			return nil, err
		}
		out = append(out, Readback{Set: setValue, Binding: bindingValue, Payload: payload})
	}
	return out, nil
}

func deviceInfoValue(v DeviceInfo) octxiliary.Value {
	return octxiliary.RecordValue(recordDeviceInfo, []octxiliary.FieldValue{
		field("RuntimeName", octxiliary.StringValue(v.RuntimeName)),
		field("RuntimeVersion", octxiliary.StringValue(v.RuntimeVersion)),
		field("KaijuUpstreamCommit", octxiliary.StringValue(v.KaijuUpstreamCommit)),
		field("KaijuForkCommit", octxiliary.StringValue(v.KaijuForkCommit)),
		field("DeviceName", octxiliary.StringValue(v.DeviceName)),
		field("VendorID", octxiliary.IntValue(int(v.VendorID))),
		field("DeviceID", octxiliary.IntValue(int(v.DeviceID))),
		field("DriverVersion", octxiliary.IntValue(int(v.DriverVersion))),
		field("VulkanAPIVersion", octxiliary.StringValue(v.VulkanAPIVersion)),
		field("TimestampPeriodNS", octxiliary.FloatValue(v.TimestampPeriodNS)),
		field("TimestampValidBits", octxiliary.IntValue(int(v.TimestampValidBits))),
		field("QueueFamilyIndex", octxiliary.IntValue(int(v.QueueFamilyIndex))),
		field("QueueFlags", octxiliary.IntValue(int(v.QueueFlags))),
		field("BufferMemoryTypeIndex", octxiliary.IntValue(int(v.BufferMemoryTypeIndex))),
		field("BufferMemoryPropertyFlags", octxiliary.IntValue(int(v.BufferMemoryPropertyFlags))),
		field("BufferUsageFlags", octxiliary.IntValue(int(v.BufferUsageFlags))),
		field("BufferSharingMode", octxiliary.StringValue(v.BufferSharingMode)),
		field("BufferMemoryAlignment", octxiliary.IntValue(int(v.BufferMemoryAlignment))),
		field("BufferMemoryOffset", octxiliary.IntValue(int(v.BufferMemoryOffset))),
		field("Headless", octxiliary.BoolValue(v.Headless)),
	})
}

func parseDeviceInfo(fields []octxiliary.FieldValue) (DeviceInfo, error) {
	indexed := fieldsByName(fields)
	vendor, err := uint32Field(indexed, "VendorID")
	if err != nil {
		return DeviceInfo{}, err
	}
	deviceID, err := uint32Field(indexed, "DeviceID")
	if err != nil {
		return DeviceInfo{}, err
	}
	driver, err := uint32Field(indexed, "DriverVersion")
	if err != nil {
		return DeviceInfo{}, err
	}
	validBits, err := uint32Field(indexed, "TimestampValidBits")
	if err != nil {
		return DeviceInfo{}, err
	}
	queueFamilyIndex, err := uint32Field(indexed, "QueueFamilyIndex")
	if err != nil {
		return DeviceInfo{}, err
	}
	queueFlags, err := uint32Field(indexed, "QueueFlags")
	if err != nil {
		return DeviceInfo{}, err
	}
	memoryTypeIndex, err := uint32Field(indexed, "BufferMemoryTypeIndex")
	if err != nil {
		return DeviceInfo{}, err
	}
	memoryFlags, err := uint32Field(indexed, "BufferMemoryPropertyFlags")
	if err != nil {
		return DeviceInfo{}, err
	}
	usageFlags, err := uint32Field(indexed, "BufferUsageFlags")
	if err != nil {
		return DeviceInfo{}, err
	}
	memoryAlignment, err := uint32Field(indexed, "BufferMemoryAlignment")
	if err != nil {
		return DeviceInfo{}, err
	}
	memoryOffset, err := uint32Field(indexed, "BufferMemoryOffset")
	if err != nil {
		return DeviceInfo{}, err
	}
	headless, err := boolField(indexed, "Headless")
	if err != nil {
		return DeviceInfo{}, err
	}
	period, err := floatField(indexed, "TimestampPeriodNS")
	if err != nil {
		return DeviceInfo{}, err
	}
	return DeviceInfo{
		RuntimeName:               stringFieldMust(indexed, "RuntimeName"),
		RuntimeVersion:            stringFieldMust(indexed, "RuntimeVersion"),
		KaijuUpstreamCommit:       stringFieldMust(indexed, "KaijuUpstreamCommit"),
		KaijuForkCommit:           stringFieldMust(indexed, "KaijuForkCommit"),
		DeviceName:                stringFieldMust(indexed, "DeviceName"),
		VendorID:                  vendor,
		DeviceID:                  deviceID,
		DriverVersion:             driver,
		VulkanAPIVersion:          stringFieldMust(indexed, "VulkanAPIVersion"),
		TimestampPeriodNS:         period,
		TimestampValidBits:        validBits,
		QueueFamilyIndex:          queueFamilyIndex,
		QueueFlags:                queueFlags,
		BufferMemoryTypeIndex:     memoryTypeIndex,
		BufferMemoryPropertyFlags: memoryFlags,
		BufferUsageFlags:          usageFlags,
		BufferSharingMode:         stringFieldMust(indexed, "BufferSharingMode"),
		BufferMemoryAlignment:     memoryAlignment,
		BufferMemoryOffset:        memoryOffset,
		Headless:                  headless,
	}, nil
}

func timingValue(v Timing) octxiliary.Value {
	samples := make([]int, 0, len(v.SamplesNS))
	for _, sample := range v.SamplesNS {
		samples = append(samples, int(sample))
	}
	return octxiliary.RecordValue(recordTiming, []octxiliary.FieldValue{
		field("Source", octxiliary.StringValue(v.Source)),
		field("StageSpan", octxiliary.StringValue(v.StageSpan)),
		field("TimestampStartStage", octxiliary.StringValue(v.TimestampStartStage)),
		field("TimestampEndStage", octxiliary.StringValue(v.TimestampEndStage)),
		field("IntervalCommands", stringListValue(v.IntervalCommands)),
		field("DispatchesPerSample", octxiliary.IntValue(int(v.DispatchesPerSample))),
		field("QueryResetLocation", octxiliary.StringValue(v.QueryResetLocation)),
		field("FenceWaitLocation", octxiliary.StringValue(v.FenceWaitLocation)),
		field("ResultRetrievalLocation", octxiliary.StringValue(v.ResultRetrievalLocation)),
		field("SamplesNS", intListValue(samples)),
	})
}

func parseTiming(fields []octxiliary.FieldValue) (Timing, error) {
	indexed := fieldsByName(fields)
	samples, err := parseIntListValue(indexed["SamplesNS"])
	if err != nil {
		return Timing{}, err
	}
	commands, err := parseStringListValue(indexed["IntervalCommands"])
	if err != nil {
		return Timing{}, err
	}
	dispatches, err := uint32Field(indexed, "DispatchesPerSample")
	if err != nil {
		return Timing{}, err
	}
	out := make([]uint64, 0, len(samples))
	for _, sample := range samples {
		if sample < 0 {
			return Timing{}, fmt.Errorf("timing sample must be non-negative")
		}
		out = append(out, uint64(sample))
	}
	return Timing{
		Source:                  stringFieldMust(indexed, "Source"),
		StageSpan:               stringFieldMust(indexed, "StageSpan"),
		TimestampStartStage:     stringFieldMust(indexed, "TimestampStartStage"),
		TimestampEndStage:       stringFieldMust(indexed, "TimestampEndStage"),
		IntervalCommands:        commands,
		DispatchesPerSample:     dispatches,
		QueryResetLocation:      stringFieldMust(indexed, "QueryResetLocation"),
		FenceWaitLocation:       stringFieldMust(indexed, "FenceWaitLocation"),
		ResultRetrievalLocation: stringFieldMust(indexed, "ResultRetrievalLocation"),
		SamplesNS:               out,
	}, nil
}

func validationValue(v ValidationStatus) octxiliary.Value {
	return octxiliary.RecordValue(recordValidation, []octxiliary.FieldValue{
		field("Requested", octxiliary.BoolValue(v.Requested)),
		field("Available", octxiliary.BoolValue(v.Available)),
		field("Enabled", octxiliary.BoolValue(v.Enabled)),
		field("Warnings", octxiliary.IntValue(v.Warnings)),
		field("Errors", octxiliary.IntValue(v.Errors)),
		field("DeviceLost", octxiliary.BoolValue(v.DeviceLost)),
	})
}

func parseValidation(fields []octxiliary.FieldValue) (ValidationStatus, error) {
	indexed := fieldsByName(fields)
	warnings, err := intField(indexed, "Warnings")
	if err != nil {
		return ValidationStatus{}, err
	}
	errorsValue, err := intField(indexed, "Errors")
	if err != nil {
		return ValidationStatus{}, err
	}
	requested, err := boolField(indexed, "Requested")
	if err != nil {
		return ValidationStatus{}, err
	}
	available, err := boolField(indexed, "Available")
	if err != nil {
		return ValidationStatus{}, err
	}
	enabled, err := boolField(indexed, "Enabled")
	if err != nil {
		return ValidationStatus{}, err
	}
	deviceLost, err := boolField(indexed, "DeviceLost")
	if err != nil {
		return ValidationStatus{}, err
	}
	return ValidationStatus{Requested: requested, Available: available, Enabled: enabled, Warnings: warnings, Errors: errorsValue, DeviceLost: deviceLost}, nil
}

func diagnosticListValue(values []Diagnostic) octxiliary.Value {
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(values)))}
	for i, value := range values {
		fields = append(fields, field("Item"+strconv.Itoa(i), octxiliary.RecordValue(recordDiagnostic, []octxiliary.FieldValue{
			field("Severity", octxiliary.StringValue(value.Severity)),
			field("Type", octxiliary.StringValue(value.Type)),
			field("MessageID", octxiliary.StringValue(value.MessageID)),
			field("Message", octxiliary.StringValue(value.Message)),
		})))
	}
	return octxiliary.RecordValue(recordDiagnosticList, fields)
}

func parseDiagnosticListValue(value octxiliary.Value) ([]Diagnostic, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordDiagnosticList {
		return nil, fmt.Errorf("expected %s record", recordDiagnosticList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	out := make([]Diagnostic, 0, count)
	for i := 0; i < count; i++ {
		item, err := recordField(indexed, "Item"+strconv.Itoa(i), recordDiagnostic)
		if err != nil {
			return nil, err
		}
		fields := fieldsByName(item.Fields)
		out = append(out, Diagnostic{
			Severity:  stringFieldMust(fields, "Severity"),
			Type:      stringFieldMust(fields, "Type"),
			MessageID: stringFieldMust(fields, "MessageID"),
			Message:   stringFieldMust(fields, "Message"),
		})
	}
	return out, nil
}

func limitsValue(v Limits) octxiliary.Value {
	return octxiliary.RecordValue(recordLimits, []octxiliary.FieldValue{
		field("MaxSPIRVBytes", octxiliary.IntValue(int(v.MaxSPIRVBytes))),
		field("MaxResources", octxiliary.IntValue(int(v.MaxResources))),
		field("MaxBytesPerResource", octxiliary.IntValue(int(v.MaxBytesPerResource))),
		field("MaxAggregatePayloadBytes", octxiliary.IntValue(int(v.MaxAggregatePayloadBytes))),
		field("MaxAggregateReadbackBytes", octxiliary.IntValue(int(v.MaxAggregateReadbackBytes))),
		field("MaxPushConstantBytes", octxiliary.IntValue(int(v.MaxPushConstantBytes))),
		field("MaxWarmup", octxiliary.IntValue(int(v.MaxWarmup))),
		field("MaxIterations", octxiliary.IntValue(int(v.MaxIterations))),
		field("MaxDispatchDimension", octxiliary.IntValue(int(v.MaxDispatchDimension))),
		field("MaxResponseBytes", octxiliary.IntValue(int(v.MaxResponseBytes))),
		field("TimeoutMS", octxiliary.IntValue(int(v.TimeoutMS))),
	})
}

func parseLimits(fields []octxiliary.FieldValue) (Limits, error) {
	indexed := fieldsByName(fields)
	maxSPIRV, err := uint32Field(indexed, "MaxSPIRVBytes")
	if err != nil {
		return Limits{}, err
	}
	maxResources, err := uint32Field(indexed, "MaxResources")
	if err != nil {
		return Limits{}, err
	}
	maxBytesPerResource, err := uint32Field(indexed, "MaxBytesPerResource")
	if err != nil {
		return Limits{}, err
	}
	maxAggregatePayload, err := uint32Field(indexed, "MaxAggregatePayloadBytes")
	if err != nil {
		return Limits{}, err
	}
	maxAggregateReadback, err := uint32Field(indexed, "MaxAggregateReadbackBytes")
	if err != nil {
		return Limits{}, err
	}
	maxPushConstants, err := uint32Field(indexed, "MaxPushConstantBytes")
	if err != nil {
		return Limits{}, err
	}
	maxWarmup, err := uint32Field(indexed, "MaxWarmup")
	if err != nil {
		return Limits{}, err
	}
	maxIterations, err := uint32Field(indexed, "MaxIterations")
	if err != nil {
		return Limits{}, err
	}
	maxDispatchDimension, err := uint32Field(indexed, "MaxDispatchDimension")
	if err != nil {
		return Limits{}, err
	}
	maxResponseBytes, err := uint32Field(indexed, "MaxResponseBytes")
	if err != nil {
		return Limits{}, err
	}
	timeoutMS, err := uint32Field(indexed, "TimeoutMS")
	if err != nil {
		return Limits{}, err
	}
	return Limits{
		MaxSPIRVBytes: maxSPIRV, MaxResources: maxResources, MaxBytesPerResource: maxBytesPerResource,
		MaxAggregatePayloadBytes: maxAggregatePayload, MaxAggregateReadbackBytes: maxAggregateReadback,
		MaxPushConstantBytes: maxPushConstants, MaxWarmup: maxWarmup, MaxIterations: maxIterations,
		MaxDispatchDimension: maxDispatchDimension, MaxResponseBytes: maxResponseBytes, TimeoutMS: timeoutMS,
	}, nil
}

func intListValue(values []int) octxiliary.Value {
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(values)))}
	for i, value := range values {
		fields = append(fields, field("Item"+strconv.Itoa(i), octxiliary.IntValue(value)))
	}
	return octxiliary.RecordValue(recordIntList, fields)
}

func parseIntListValue(value octxiliary.Value) ([]int, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordIntList {
		return nil, fmt.Errorf("expected %s record", recordIntList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	out := make([]int, 0, count)
	for i := 0; i < count; i++ {
		item, err := intField(indexed, "Item"+strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func stringListValue(values []string) octxiliary.Value {
	fields := []octxiliary.FieldValue{field("Count", octxiliary.IntValue(len(values)))}
	for i, value := range values {
		fields = append(fields, field("Item"+strconv.Itoa(i), octxiliary.StringValue(value)))
	}
	return octxiliary.RecordValue(recordStringList, fields)
}

func parseStringListValue(value octxiliary.Value) ([]string, error) {
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordStringList {
		return nil, fmt.Errorf("expected %s record", recordStringList)
	}
	indexed := fieldsByName(value.Fields)
	count, err := intField(indexed, "Count")
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, count)
	for i := 0; i < count; i++ {
		item, err := stringField(indexed, "Item"+strconv.Itoa(i))
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func field(name string, value octxiliary.Value) octxiliary.FieldValue {
	return octxiliary.FieldValue{Name: name, Value: value}
}

func fieldsByName(fields []octxiliary.FieldValue) map[string]octxiliary.Value {
	indexed := make(map[string]octxiliary.Value, len(fields))
	for _, value := range fields {
		indexed[value.Name] = value.Value
	}
	return indexed
}

func recordField(indexed map[string]octxiliary.Value, name, recordType string) (octxiliary.Value, error) {
	value, ok := indexed[name]
	if !ok {
		return octxiliary.Value{}, fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueRecord || value.RecordType != recordType {
		return octxiliary.Value{}, fmt.Errorf("field %s expects %s record", name, recordType)
	}
	return value, nil
}

func intField(indexed map[string]octxiliary.Value, name string) (int, error) {
	value, ok := indexed[name]
	if !ok {
		return 0, fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueInt {
		return 0, fmt.Errorf("field %s expects Int, got %s", name, value.Kind)
	}
	return value.Int, nil
}

func uint32Field(indexed map[string]octxiliary.Value, name string) (uint32, error) {
	value, err := intField(indexed, name)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("field %s must be non-negative", name)
	}
	return uint32(value), nil
}

func floatField(indexed map[string]octxiliary.Value, name string) (float64, error) {
	value, ok := indexed[name]
	if !ok {
		return 0, fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueFloat {
		return 0, fmt.Errorf("field %s expects Float, got %s", name, value.Kind)
	}
	return value.Float, nil
}

func boolField(indexed map[string]octxiliary.Value, name string) (bool, error) {
	value, ok := indexed[name]
	if !ok {
		return false, fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueBool {
		return false, fmt.Errorf("field %s expects Bool, got %s", name, value.Kind)
	}
	return value.Bool, nil
}

func boolFieldMust(indexed map[string]octxiliary.Value, name string) bool {
	value, _ := boolField(indexed, name)
	return value
}

func stringField(indexed map[string]octxiliary.Value, name string) (string, error) {
	value, ok := indexed[name]
	if !ok {
		return "", fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueString {
		return "", fmt.Errorf("field %s expects String, got %s", name, value.Kind)
	}
	return value.String, nil
}

func stringFieldMust(indexed map[string]octxiliary.Value, name string) string {
	value, _ := stringField(indexed, name)
	return value
}

func bytesField(indexed map[string]octxiliary.Value, name string) ([]byte, error) {
	value, ok := indexed[name]
	if !ok {
		return nil, fmt.Errorf("missing field %s", name)
	}
	if value.Kind != octxiliary.ValueBytes {
		return nil, fmt.Errorf("field %s expects Bytes, got %s", name, value.Kind)
	}
	return append([]byte(nil), value.Bytes...), nil
}

func CapabilitySummary(v Capabilities) string {
	return strings.Join([]string{
		fmt.Sprintf("protocol=%d", v.Protocol),
		fmt.Sprintf("dispatch=%t", v.DispatchSupported),
		fmt.Sprintf("benchmark=%t", v.BenchmarkSupported),
		fmt.Sprintf("device=%s", v.Device.DeviceName),
	}, " ")
}

package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"
	"unsafe"

	vk "kaijuengine.com/rendering/vulkan"
	vc "kaijuengine.com/rendering/vulkan_const"
)

const kaijuCommit = "ed509b23ed2b230fefe1c6c4ed00f9fa27315ab2"

type request struct {
	SchemaVersion       int        `json:"schemaVersion"`
	Operation           string     `json:"operation,omitempty"`
	SPIRVPath           string     `json:"spirvPath"`
	SPIRVHash           string     `json:"spirvHash"`
	EntryPoint          string     `json:"entryPoint"`
	WorkgroupSize       [3]uint32  `json:"workgroupSize"`
	DispatchGroups      [3]uint32  `json:"dispatchGroups"`
	PushConstantsBase64 string     `json:"pushConstantsBase64,omitempty"`
	Resources           []resource `json:"resources"`
	Warmup              int        `json:"warmup"`
	Iterations          int        `json:"iterations"`
}

type resource struct {
	Set           uint32 `json:"set"`
	Binding       uint32 `json:"binding"`
	Access        string `json:"access"`
	Kind          string `json:"kind,omitempty"`
	ElementType   string `json:"elementType"`
	ByteLength    int    `json:"byteLength"`
	PayloadBase64 string `json:"payloadBase64"`
	Readback      bool   `json:"readback"`
}

type runtimeInfo struct {
	Name      string `json:"name"`
	Commit    string `json:"commit"`
	Device    string `json:"device"`
	Driver    uint32 `json:"driver"`
	VulkanAPI string `json:"vulkanApi"`
	GoVersion string `json:"goVersion"`
	Timing    string `json:"timingSource"`
	Headless  bool   `json:"headless"`
}

type readback struct {
	Set           uint32 `json:"set"`
	Binding       uint32 `json:"binding"`
	PayloadBase64 string `json:"payloadBase64"`
}

type response struct {
	SchemaVersion int         `json:"schemaVersion"`
	Success       bool        `json:"success"`
	Runtime       runtimeInfo `json:"runtime"`
	SPIRVHash     string      `json:"spirvHash,omitempty"`
	SamplesNS     []uint64    `json:"samplesNs"`
	Readbacks     []readback  `json:"readbacks"`
	Warnings      []string    `json:"warnings"`
	Errors        []string    `json:"errors"`
}

type buffer struct {
	request resource
	handle  vk.Buffer
	memory  vk.DeviceMemory
	data    unsafe.Pointer
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
}

func main() {
	requestPath := flag.String("request", "", "path to a compute.dispatch JSON request")
	flag.Parse()
	resp := response{SchemaVersion: 1, SamplesNS: []uint64{}, Readbacks: []readback{}, Warnings: []string{}, Errors: []string{}}
	if *requestPath == "" || flag.NArg() != 0 {
		resp.Errors = append(resp.Errors, "usage: octxiliary-kaiju-vulkan --request request.json")
		writeResponse(resp)
		os.Exit(2)
	}
	data, err := os.ReadFile(*requestPath)
	if err != nil {
		resp.Errors = append(resp.Errors, err.Error())
		writeResponse(resp)
		os.Exit(1)
	}
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		resp.Errors = append(resp.Errors, fmt.Sprintf("decode request: %v", err))
		writeResponse(resp)
		os.Exit(1)
	}
	resp, err = dispatch(req)
	if err != nil {
		resp.Success = false
		resp.Errors = append(resp.Errors, err.Error())
	}
	writeResponse(resp)
	if err != nil {
		os.Exit(1)
	}
}

func writeResponse(resp response) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(resp)
}

func dispatch(req request) (resp response, err error) {
	resp = response{SchemaVersion: 1, SamplesNS: []uint64{}, Readbacks: []readback{}, Warnings: []string{}, Errors: []string{}}
	if err := validateRequest(req); err != nil {
		return resp, err
	}
	spv, err := os.ReadFile(req.SPIRVPath)
	if err != nil {
		return resp, fmt.Errorf("read SPIR-V: %w", err)
	}
	sum := sha256.Sum256(spv)
	actualHash := hex.EncodeToString(sum[:])
	resp.SPIRVHash = actualHash
	if len(req.SPIRVHash) == 64 && !strings.EqualFold(req.SPIRVHash, actualHash) {
		return resp, fmt.Errorf("SPIR-V hash mismatch: request %s, actual %s", req.SPIRVHash, actualHash)
	}
	push, err := base64.StdEncoding.DecodeString(req.PushConstantsBase64)
	if err != nil {
		return resp, fmt.Errorf("decode push constants: %w", err)
	}
	c := &context{}
	defer c.destroy()
	if err := c.initialize(req, spv, push); err != nil {
		return resp, err
	}
	resp.Runtime = runtimeInfo{
		Name: "kaiju", Commit: kaijuCommit, Device: vk.ToString(c.properties.DeviceName[:]),
		Driver: c.properties.DriverVersion, VulkanAPI: vk.Version(c.properties.ApiVersion).String(),
		GoVersion: runtime.Version(), Timing: "vulkan_query_pool_gpu_timestamp", Headless: true,
	}
	for i := 0; i < req.Warmup+req.Iterations; i++ {
		ns, err := c.runOnce(req.DispatchGroups, push)
		if err != nil {
			return resp, err
		}
		if i >= req.Warmup {
			resp.SamplesNS = append(resp.SamplesNS, ns)
		}
	}
	for _, b := range c.buffers {
		if !b.request.Readback {
			continue
		}
		bytes := unsafe.Slice((*byte)(b.data), b.request.ByteLength)
		resp.Readbacks = append(resp.Readbacks, readback{Set: b.request.Set, Binding: b.request.Binding, PayloadBase64: base64.StdEncoding.EncodeToString(bytes)})
	}
	resp.Success = true
	return resp, nil
}

func validateRequest(req request) error {
	if req.SchemaVersion != 1 {
		return fmt.Errorf("unsupported schemaVersion %d", req.SchemaVersion)
	}
	if req.Operation != "" && req.Operation != "compute.dispatch" {
		return fmt.Errorf("unsupported operation %q", req.Operation)
	}
	if req.SPIRVPath == "" || req.EntryPoint == "" {
		return errors.New("spirvPath and entryPoint are required")
	}
	if req.DispatchGroups[0] == 0 || req.DispatchGroups[1] == 0 || req.DispatchGroups[2] == 0 {
		return errors.New("dispatchGroups must be non-zero")
	}
	if req.Warmup < 0 || req.Iterations < 1 {
		return errors.New("warmup must be non-negative and iterations must be positive")
	}
	seen := map[uint32]bool{}
	for _, r := range req.Resources {
		if r.Set != 0 || r.ByteLength <= 0 {
			return fmt.Errorf("only positive-length set 0 storage buffers are supported (set=%d binding=%d)", r.Set, r.Binding)
		}
		if r.Kind != "" && r.Kind != "storage_buffer" {
			return fmt.Errorf("binding %d has unsupported kind %q", r.Binding, r.Kind)
		}
		if seen[r.Binding] {
			return fmt.Errorf("duplicate binding %d", r.Binding)
		}
		seen[r.Binding] = true
	}
	return nil
}

func (c *context) initialize(req request, spv, push []byte) error {
	if err := vk.SetDefaultGetInstanceProcAddr(); err != nil {
		return err
	}
	if err := vk.Init(); err != nil {
		return err
	}
	appName := append([]byte("oct-kaiju-spike"), 0)
	engineName := append([]byte("kaiju-raw-vulkan"), 0)
	app := vk.ApplicationInfo{SType: vc.StructureTypeApplicationInfo, PApplicationName: (*vk.Char)(unsafe.Pointer(&appName[0])), ApplicationVersion: vk.MakeVersion(0, 1, 0), PEngineName: (*vk.Char)(unsafe.Pointer(&engineName[0])), EngineVersion: 1, ApiVersion: vk.MakeVersion(1, 0, 0)}
	ici := vk.InstanceCreateInfo{SType: vc.StructureTypeInstanceCreateInfo, PApplicationInfo: &app}
	if err := check("vkCreateInstance", vk.CreateInstance(&ici, nil, &c.instance)); err != nil {
		return err
	}
	if err := vk.InitInstance(c.instance); err != nil {
		return err
	}
	if err := c.selectDevice(); err != nil {
		return err
	}
	priority := float32(1)
	qci := vk.DeviceQueueCreateInfo{SType: vc.StructureTypeDeviceQueueCreateInfo, QueueFamilyIndex: c.queueFamily, QueueCount: 1, PQueuePriorities: &priority}
	dci := vk.DeviceCreateInfo{SType: vc.StructureTypeDeviceCreateInfo, QueueCreateInfoCount: 1, PQueueCreateInfos: &qci}
	if err := check("vkCreateDevice", vk.CreateDevice(c.physical, &dci, nil, &c.device)); err != nil {
		return err
	}
	vk.GetDeviceQueue(c.device, c.queueFamily, 0, &c.queue)
	for _, r := range req.Resources {
		if err := c.createBuffer(r); err != nil {
			return err
		}
	}
	if err := c.createDescriptors(); err != nil {
		return err
	}
	if err := c.createPipeline(req.EntryPoint, spv, len(push)); err != nil {
		return err
	}
	if err := c.createCommands(); err != nil {
		return err
	}
	return nil
}

func (c *context) selectDevice() error {
	var count uint32
	if err := check("vkEnumeratePhysicalDevices(count)", vk.EnumeratePhysicalDevices(c.instance, &count, nil)); err != nil {
		return err
	}
	if count == 0 {
		return errors.New("no Vulkan physical devices")
	}
	devices := make([]vk.PhysicalDevice, count)
	if err := check("vkEnumeratePhysicalDevices", vk.EnumeratePhysicalDevices(c.instance, &count, &devices[0])); err != nil {
		return err
	}
	for _, d := range devices {
		var qcount uint32
		vk.GetPhysicalDeviceQueueFamilyProperties(d, &qcount, nil)
		queues := make([]vk.QueueFamilyProperties, qcount)
		vk.GetPhysicalDeviceQueueFamilyProperties(d, &qcount, &queues[0])
		for i, q := range queues {
			if q.QueueCount > 0 && q.QueueFlags&vk.QueueFlags(vc.QueueComputeBit) != 0 && q.TimestampValidBits > 0 {
				c.physical, c.queueFamily = d, uint32(i)
				vk.GetPhysicalDeviceProperties(d, &c.properties)
				vk.GetPhysicalDeviceMemoryProperties(d, &c.memoryProps)
				return nil
			}
		}
	}
	return errors.New("no compute queue with timestamp support")
}

func (c *context) createBuffer(r resource) error {
	bci := vk.BufferCreateInfo{SType: vc.StructureTypeBufferCreateInfo, Size: vk.DeviceSize(r.ByteLength), Usage: vk.BufferUsageFlags(vc.BufferUsageStorageBufferBit), SharingMode: vc.SharingModeExclusive}
	b := buffer{request: r}
	if err := check("vkCreateBuffer", vk.CreateBuffer(c.device, &bci, nil, &b.handle)); err != nil {
		return err
	}
	var mr vk.MemoryRequirements
	vk.GetBufferMemoryRequirements(c.device, b.handle, &mr)
	idx, ok := c.memoryType(mr.MemoryTypeBits, vk.MemoryPropertyFlags(vc.MemoryPropertyHostVisibleBit|vc.MemoryPropertyHostCoherentBit))
	if !ok {
		return fmt.Errorf("binding %d: no host-visible coherent memory type", r.Binding)
	}
	mai := vk.MemoryAllocateInfo{SType: vc.StructureTypeMemoryAllocateInfo, AllocationSize: mr.Size, MemoryTypeIndex: idx}
	if err := check("vkAllocateMemory", vk.AllocateMemory(c.device, &mai, nil, &b.memory)); err != nil {
		return err
	}
	if err := check("vkBindBufferMemory", vk.BindBufferMemory(c.device, b.handle, b.memory, 0)); err != nil {
		return err
	}
	if err := check("vkMapMemory", vk.MapMemory(c.device, b.memory, 0, vk.DeviceSize(r.ByteLength), 0, &b.data)); err != nil {
		return err
	}
	payload, err := base64.StdEncoding.DecodeString(r.PayloadBase64)
	if err != nil {
		return fmt.Errorf("binding %d payload: %w", r.Binding, err)
	}
	if len(payload) > r.ByteLength {
		return fmt.Errorf("binding %d payload is %d bytes, byteLength is %d", r.Binding, len(payload), r.ByteLength)
	}
	dst := unsafe.Slice((*byte)(b.data), r.ByteLength)
	clear(dst)
	copy(dst, payload)
	c.buffers = append(c.buffers, b)
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

func (c *context) createDescriptors() error {
	if len(c.buffers) == 0 {
		return nil
	}
	sort.Slice(c.buffers, func(i, j int) bool { return c.buffers[i].request.Binding < c.buffers[j].request.Binding })
	bindings := make([]vk.DescriptorSetLayoutBinding, len(c.buffers))
	for i, b := range c.buffers {
		bindings[i] = vk.DescriptorSetLayoutBinding{Binding: b.request.Binding, DescriptorType: vc.DescriptorTypeStorageBuffer, DescriptorCount: 1, StageFlags: vk.ShaderStageFlags(vc.ShaderStageComputeBit)}
	}
	lci := vk.DescriptorSetLayoutCreateInfo{SType: vc.StructureTypeDescriptorSetLayoutCreateInfo, BindingCount: uint32(len(bindings)), PBindings: &bindings[0]}
	if err := check("vkCreateDescriptorSetLayout", vk.CreateDescriptorSetLayout(c.device, &lci, nil, &c.setLayout)); err != nil {
		return err
	}
	ps := vk.DescriptorPoolSize{Type: vc.DescriptorTypeStorageBuffer, DescriptorCount: uint32(len(bindings))}
	pci := vk.DescriptorPoolCreateInfo{SType: vc.StructureTypeDescriptorPoolCreateInfo, MaxSets: 1, PoolSizeCount: 1, PPoolSizes: &ps}
	if err := check("vkCreateDescriptorPool", vk.CreateDescriptorPool(c.device, &pci, nil, &c.descriptorPool)); err != nil {
		return err
	}
	dai := vk.DescriptorSetAllocateInfo{SType: vc.StructureTypeDescriptorSetAllocateInfo, DescriptorPool: c.descriptorPool, DescriptorSetCount: 1, PSetLayouts: &c.setLayout}
	if err := check("vkAllocateDescriptorSets", vk.AllocateDescriptorSets(c.device, &dai, &c.descriptorSet)); err != nil {
		return err
	}
	infos := make([]vk.DescriptorBufferInfo, len(c.buffers))
	writes := make([]vk.WriteDescriptorSet, len(c.buffers))
	for i, b := range c.buffers {
		infos[i] = vk.DescriptorBufferInfo{Buffer: b.handle, Range: vk.DeviceSize(b.request.ByteLength)}
		writes[i] = vk.WriteDescriptorSet{SType: vc.StructureTypeWriteDescriptorSet, DstSet: c.descriptorSet, DstBinding: b.request.Binding, DescriptorCount: 1, DescriptorType: vc.DescriptorTypeStorageBuffer, PBufferInfo: &infos[i]}
	}
	var pinner runtime.Pinner
	pinner.Pin(&infos[0])
	defer pinner.Unpin()
	vk.UpdateDescriptorSets(c.device, uint32(len(writes)), &writes[0], 0, nil)
	runtime.KeepAlive(infos)
	runtime.KeepAlive(writes)
	return nil
}

func (c *context) createPipeline(entry string, spv []byte, pushBytes int) error {
	if len(spv)%4 != 0 {
		return errors.New("SPIR-V byte length is not divisible by four")
	}
	words := make([]uint32, len(spv)/4)
	for i := range words {
		words[i] = binary.LittleEndian.Uint32(spv[i*4:])
	}
	sci := vk.ShaderModuleCreateInfo{SType: vc.StructureTypeShaderModuleCreateInfo, CodeSize: uint(len(spv)), PCode: &words[0]}
	if err := check("vkCreateShaderModule", vk.CreateShaderModule(c.device, &sci, nil, &c.shader)); err != nil {
		return err
	}
	plci := vk.PipelineLayoutCreateInfo{SType: vc.StructureTypePipelineLayoutCreateInfo}
	if c.setLayout != nil {
		plci.SetLayoutCount, plci.PSetLayouts = 1, &c.setLayout
	}
	var pcr vk.PushConstantRange
	if pushBytes > 0 {
		if pushBytes%4 != 0 {
			return errors.New("push constants length must be divisible by four")
		}
		pcr = vk.PushConstantRange{StageFlags: vk.ShaderStageFlags(vc.ShaderStageComputeBit), Size: uint32(pushBytes)}
		plci.PushConstantRangeCount, plci.PPushConstantRanges = 1, &pcr
	}
	if err := check("vkCreatePipelineLayout", vk.CreatePipelineLayout(c.device, &plci, nil, &c.pipelineLayout)); err != nil {
		return err
	}
	name := append([]byte(entry), 0)
	stage := vk.PipelineShaderStageCreateInfo{SType: vc.StructureTypePipelineShaderStageCreateInfo, Stage: vc.ShaderStageComputeBit, Module: c.shader, PName: (*vk.Char)(unsafe.Pointer(&name[0]))}
	cpci := vk.ComputePipelineCreateInfo{SType: vc.StructureTypeComputePipelineCreateInfo, Stage: stage, Layout: c.pipelineLayout, BasePipelineIndex: -1}
	return check("vkCreateComputePipelines", vk.CreateComputePipelines(c.device, nil, 1, &cpci, nil, &c.pipeline))
}

func (c *context) createCommands() error {
	cpci := vk.CommandPoolCreateInfo{SType: vc.StructureTypeCommandPoolCreateInfo, Flags: vk.CommandPoolCreateFlags(vc.CommandPoolCreateResetCommandBufferBit), QueueFamilyIndex: c.queueFamily}
	if err := check("vkCreateCommandPool", vk.CreateCommandPool(c.device, &cpci, nil, &c.commandPool)); err != nil {
		return err
	}
	cai := vk.CommandBufferAllocateInfo{SType: vc.StructureTypeCommandBufferAllocateInfo, CommandPool: c.commandPool, Level: vc.CommandBufferLevelPrimary, CommandBufferCount: 1}
	if err := check("vkAllocateCommandBuffers", vk.AllocateCommandBuffers(c.device, &cai, &c.commandBuffer)); err != nil {
		return err
	}
	fci := vk.FenceCreateInfo{SType: vc.StructureTypeFenceCreateInfo}
	if err := check("vkCreateFence", vk.CreateFence(c.device, &fci, nil, &c.fence)); err != nil {
		return err
	}
	qci := vk.QueryPoolCreateInfo{SType: vc.StructureTypeQueryPoolCreateInfo, QueryType: vc.QueryTypeTimestamp, QueryCount: 2}
	return check("vkCreateQueryPool", vk.CreateQueryPool(c.device, &qci, nil, &c.queryPool))
}

func (c *context) runOnce(groups [3]uint32, push []byte) (uint64, error) {
	if err := check("vkResetFences", vk.ResetFences(c.device, 1, &c.fence)); err != nil {
		return 0, err
	}
	if err := check("vkResetCommandBuffer", vk.ResetCommandBuffer(c.commandBuffer, 0)); err != nil {
		return 0, err
	}
	bi := vk.CommandBufferBeginInfo{SType: vc.StructureTypeCommandBufferBeginInfo, Flags: vk.CommandBufferUsageFlags(vc.CommandBufferUsageOneTimeSubmitBit)}
	if err := check("vkBeginCommandBuffer", vk.BeginCommandBuffer(c.commandBuffer, &bi)); err != nil {
		return 0, err
	}
	vk.CmdResetQueryPool(c.commandBuffer, c.queryPool, 0, 2)
	vk.CmdWriteTimestamp(c.commandBuffer, vc.PipelineStageTopOfPipeBit, c.queryPool, 0)
	vk.CmdBindPipeline(c.commandBuffer, vc.PipelineBindPointCompute, c.pipeline)
	if c.descriptorSet != nil {
		vk.CmdBindDescriptorSets(c.commandBuffer, vc.PipelineBindPointCompute, c.pipelineLayout, 0, 1, &c.descriptorSet, 0, nil)
	}
	if len(push) > 0 {
		vk.CmdPushConstants(c.commandBuffer, c.pipelineLayout, vk.ShaderStageFlags(vc.ShaderStageComputeBit), 0, uint32(len(push)), unsafe.Pointer(&push[0]))
	}
	vk.CmdDispatch(c.commandBuffer, groups[0], groups[1], groups[2])
	vk.CmdWriteTimestamp(c.commandBuffer, vc.PipelineStageBottomOfPipeBit, c.queryPool, 1)
	if err := check("vkEndCommandBuffer", vk.EndCommandBuffer(c.commandBuffer)); err != nil {
		return 0, err
	}
	si := vk.SubmitInfo{SType: vc.StructureTypeSubmitInfo, CommandBufferCount: 1, PCommandBuffers: &c.commandBuffer}
	if err := check("vkQueueSubmit", vk.QueueSubmit(c.queue, 1, &si, c.fence)); err != nil {
		return 0, err
	}
	if err := check("vkWaitForFences", vk.WaitForFences(c.device, 1, &c.fence, vc.True, uint64((30*time.Second).Nanoseconds()))); err != nil {
		return 0, err
	}
	values := [2]uint64{}
	flags := vk.QueryResultFlags(vc.QueryResult64Bit | vc.QueryResultWaitBit)
	if err := check("vkGetQueryPoolResults", vk.GetQueryPoolResults(c.device, c.queryPool, 0, 2, uint(unsafe.Sizeof(values)), unsafe.Pointer(&values[0]), vk.DeviceSize(unsafe.Sizeof(values[0])), flags)); err != nil {
		return 0, err
	}
	return uint64(float64(values[1]-values[0]) * float64(c.properties.Limits.TimestampPeriod)), nil
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
	for _, b := range c.buffers {
		if b.data != nil {
			vk.UnmapMemory(c.device, b.memory)
		}
		if b.handle != nil {
			vk.DestroyBuffer(c.device, b.handle, nil)
		}
		if b.memory != nil {
			vk.FreeMemory(c.device, b.memory, nil)
		}
	}
	if c.device != nil {
		vk.DestroyDevice(c.device, nil)
	}
	if c.instance != nil {
		vk.DestroyInstance(c.instance, nil)
	}
}

func check(op string, result vc.Result) error {
	if result != vc.Success {
		return fmt.Errorf("%s failed with Vulkan result %d", op, result)
	}
	return nil
}

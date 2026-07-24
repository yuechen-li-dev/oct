// Package conceptvulkan implements the intentionally small M1 host-mechanism
// compiler. It is not a general Concept implementation.
package conceptvulkan

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const CompilerID = "concept-vulkan-m1"

type Span struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}
type Diagnostic struct {
	Code, Message string
	Span          Span
}

func (d Diagnostic) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", "concept", d.Span.Line, d.Span.Column, d.Code, d.Message)
}

type Operation struct {
	Name string
	Span Span
}
type Program struct {
	Path, Function string
	Operations     []Operation
}

var fnRE = regexp.MustCompile(`^Result<ProbeEvidence, PrometheusError> ([A-Za-z_][A-Za-z0-9_]*)\(borrow MechanismContext context, unsafe imported borrow AccelerationStructure admittedTlas\) \{$`)
var localRE = regexp.MustCompile(`^owned\s+[A-Za-z_][A-Za-z0-9_]*(?:<[^>]+>)?\s+([A-Za-z_][A-Za-z0-9_]*)\s*=`)

// Token is a source-facing token with stable line/column provenance. The M1
// parser deliberately consumes only the bounded declaration/statement forms.
type Token struct {
	Lexeme string
	Span   Span
}

func Lex(text string) ([]Token, error) {
	var tokens []Token
	line, column := 1, 1
	for i := 0; i < len(text); {
		c := text[i]
		if c == '\n' {
			line++
			column = 1
			i++
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' {
			column++
			i++
			continue
		}
		if c == '/' && i+1 < len(text) && text[i+1] == '/' {
			for i < len(text) && text[i] != '\n' {
				i++
				column++
			}
			continue
		}
		start, j := Span{line, column}, i
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
			for j < len(text) && ((text[j] >= 'a' && text[j] <= 'z') || (text[j] >= 'A' && text[j] <= 'Z') || (text[j] >= '0' && text[j] <= '9') || text[j] == '_') {
				j++
			}
		} else if c >= '0' && c <= '9' {
			for j < len(text) && text[j] >= '0' && text[j] <= '9' {
				j++
			}
		} else if c == '"' {
			j++
			for j < len(text) && text[j] != '"' {
				if text[j] == '\n' {
					return nil, Diagnostic{"CV1000", "unterminated string literal", start}
				}
				j++
			}
			if j == len(text) {
				return nil, Diagnostic{"CV1000", "unterminated string literal", start}
			}
			j++
		} else if c == '-' && j+1 < len(text) && text[j+1] == '>' {
			j += 2
		} else if strings.ContainsRune("(){};,.?<>:=", rune(c)) {
			j++
		} else {
			return nil, Diagnostic{"CV1000", fmt.Sprintf("invalid token %q", c), start}
		}
		tokens = append(tokens, Token{Lexeme: text[i:j], Span: start})
		column += j - i
		i = j
	}
	return tokens, nil
}

// Parse accepts only the M1 grammar. Its line-oriented shape is deliberate:
// no general expression language or embedded C is admitted by this profile.
func Parse(path, text string) (Program, error) {
	if _, err := Lex(text); err != nil {
		return Program{}, err
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "profile Vulkan;" {
		return Program{}, Diagnostic{"CV1001", "expected `profile Vulkan;`", Span{1, 1}}
	}
	p := Program{Path: filepath.ToSlash(path)}
	headerStart, headerEnd := -1, -1
	var header []string
	for i, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") || i == 0 || line == "import Prometheus.Vulkan;" {
			continue
		}
		if headerStart < 0 {
			headerStart = i
		}
		header = append(header, line)
		if strings.Contains(line, "{") {
			headerEnd = i
			break
		}
	}
	if headerStart < 0 || headerEnd < 0 {
		return Program{}, Diagnostic{"CV1002", "missing M1 function declaration", Span{1, 1}}
	}
	headerText := normalize(strings.Join(header, " "))
	headerSpan := lineSpan(lines[headerStart], headerStart)
	if strings.HasPrefix(headerText, "fn ") || strings.Contains(headerText, "->") || strings.Contains(headerText, ": ") {
		return Program{}, Diagnostic{"CV1005", "Concept/Vulkan uses C++-shaped typed declarations, not fn/:/-> syntax", headerSpan}
	}
	m := fnRE.FindStringSubmatch(headerText)
	if m == nil {
		return Program{}, Diagnostic{"CV1002", "expected the bounded C++-shaped M1 function declaration", headerSpan}
	}
	if !pascal(m[1]) {
		return Program{}, Diagnostic{"CV1201", "function names must be PascalCase", headerSpan}
	}
	p.Function = m[1]
	var statement []string
	statementSpan := Span{}
	for i := headerEnd + 1; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if line == "}" {
			if len(statement) == 0 {
				break
			}
			return Program{}, Diagnostic{"CV1004", "unterminated M1 statement", statementSpan}
		}
		if len(statement) == 0 {
			statementSpan = lineSpan(raw, i)
		}
		statement = append(statement, line)
		if !strings.HasSuffix(line, ";") {
			continue
		}
		joined := normalize(strings.Join(statement, " "))
		statement = nil
		if strings.HasPrefix(joined, "let ") || strings.HasPrefix(joined, "var ") {
			return Program{}, Diagnostic{"CV1005", "Concept/Vulkan locals require an explicit type, never let or var", statementSpan}
		}
		if strings.Contains(joined, "concept ") || strings.Contains(joined, "decide") || strings.Contains(joined, "yield") || strings.Contains(joined, "HLSL") {
			return Program{}, Diagnostic{"CV1003", "unsupported Concept/Vulkan construct", statementSpan}
		}
		if local := localRE.FindStringSubmatch(joined); local != nil && !camel(local[1]) {
			return Program{}, Diagnostic{"CV1202", "owned local names must be camelCase", statementSpan}
		}
		name := operationName(joined)
		if name == "" {
			return Program{}, Diagnostic{"CV1004", "unsupported or malformed M1 statement", statementSpan}
		}
		p.Operations = append(p.Operations, Operation{Name: name, Span: statementSpan})
	}
	return p, Validate(p)
}
func lineSpan(raw string, zeroLine int) Span {
	return Span{zeroLine + 1, len(raw) - len(strings.TrimLeft(raw, " \t")) + 1}
}
func normalize(s string) string {
	return strings.NewReplacer("( ", "(", " ,", ",").Replace(strings.Join(strings.Fields(s), " "))
}
func pascal(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' && !strings.Contains(s, "_")
}
func camel(s string) bool {
	return len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' && !strings.Contains(s, "_")
}
func operationName(s string) string {
	for prefix, name := range map[string]string{
		"owned MappedEvidenceBuffer evidence = CreateMappedEvidenceBuffer(context)?;": "create_buffer", "owned ComputePipeline pipeline = CreatePackagePipeline(context, \"prometheus.core@1\", \"kernel-54-default\")?;": "create_pipeline", "owned DescriptorSet descriptors = BindDescriptor(context, admittedTlas, evidence, 0, 1)?;": "bind_descriptor", "owned CommandRecording command = BeginCommands(context)?;": "begin_recording", "DeclareAccess(admittedTlas, AccelerationStructureRead);": "declare_tlas_access", "DeclareAccess(evidence, ShaderWrite);": "declare_evidence_access", "Dispatch(command, 1, 1, 1);": "dispatch", "owned Submission submission = SubmitAndWait(context, move command)?;": "submit_wait", "ReadObservation(evidence);": "observe", "return Ok;": "return",
	} {
		if s == prefix {
			return name
		}
	}
	return ""
}

var required = []string{"create_buffer", "create_pipeline", "bind_descriptor", "begin_recording", "declare_tlas_access", "declare_evidence_access", "dispatch", "submit_wait", "observe", "return"}

func Validate(p Program) error {
	if len(p.Operations) != len(required) {
		return Diagnostic{"CV2001", "M1 requires exactly the kernel-54 operation sequence", Span{1, 1}}
	}
	for i, want := range required {
		if p.Operations[i].Name != want {
			return Diagnostic{"CV2002", fmt.Sprintf("expected %s before %s", want, p.Operations[i].Name), p.Operations[i].Span}
		}
	}
	return nil
}

type MIR struct {
	Profile    string  `json:"profile"`
	Function   string  `json:"function"`
	Operations []MIROp `json:"operations"`
}
type MIROp struct {
	ID, Opcode, Ownership, Fallibility, Mapping string
	Span                                        Span `json:"span"`
}

func Lower(p Program) MIR {
	mapToC := map[string]string{"create_buffer": "prom_vk_create_buffer", "create_pipeline": "prom_reactor_runtime_get_shader_package; prom_shader_package_create_module; vkCreateComputePipelines", "bind_descriptor": "vkUpdateDescriptorSets bindings 0/1", "begin_recording": "prom_ray_begin_command", "declare_tlas_access": "kernel-54 acceleration_structure_read", "declare_evidence_access": "kernel-54 shader_write -> host_read completion", "dispatch": "vkCmdDispatch(1, 1, 1)", "submit_wait": "prom_ray_end_submit_and_free", "observe": "memcpy mapped evidence", "return": "reverse initialized cleanup"}
	ownership := map[string]string{"create_buffer": "owned evidence initialized", "create_pipeline": "owned pipeline initialized", "bind_descriptor": "owned descriptors initialized", "begin_recording": "owned command initialized", "submit_wait": "command moved; submission completed", "observe": "borrow evidence", "return": "drop descriptors, pipeline, evidence"}
	fallible := map[string]string{"create_buffer": "yes", "create_pipeline": "yes", "bind_descriptor": "yes", "begin_recording": "yes", "submit_wait": "yes"}
	m := MIR{Profile: "Vulkan", Function: p.Function}
	for i, o := range p.Operations {
		f := "no"
		if fallible[o.Name] != "" {
			f = "yes"
		}
		m.Operations = append(m.Operations, MIROp{fmt.Sprintf("m1.%02d", i+1), o.Name, ownership[o.Name], f, mapToC[o.Name], o.Span})
	}
	return m
}
func MIRText(m MIR) string {
	var b strings.Builder
	for _, o := range m.Operations {
		fmt.Fprintf(&b, "%s %s ownership=%q fallible=%s source=%d:%d maps=%q\n", o.ID, o.Opcode, o.Ownership, o.Fallibility, o.Span.Line, o.Span.Column, o.Mapping)
	}
	return b.String()
}

type Outputs map[string][]byte

func Generate(p Program, source []byte) (Outputs, error) {
	m := Lower(p)
	mir, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	mir = append(mir, '\n')
	h := []byte("/* Generated by concept-vulkan-m1. DO NOT EDIT. */\n#ifndef PROM_CONCEPT_VULKAN_KERNEL54_GENERATED_H\n#define PROM_CONCEPT_VULKAN_KERNEL54_GENERATED_H\n#include \"reactor_vulkan.h\"\n#include \"reactor_shader_package.h\"\n\n/* Private conformance-only ABI; never included by public Prometheus headers. */\nint prom_concept_vulkan_kernel54_execute(const prom_vk_runtime_services* services, prom_shader_package* package, VkAccelerationStructureKHR admitted_tlas, uint32_t* out_evidence);\n#ifdef PROM_CONCEPT_VULKAN_CONFORMANCE\nint prom_concept_vulkan_kernel54_handwritten_adapter(void* handle, uint64_t scene_id, PrometheusRayQueryProbeResult* out_result);\nint prom_concept_vulkan_kernel54_generated_adapter(void* handle, uint64_t scene_id, PrometheusRayQueryProbeResult* out_result);\n#endif\n\n#endif\n")
	c := []byte(`/* Generated by concept-vulkan-m1. DO NOT EDIT. Source: Examples/Concept-Vulkan/kernel54_probe.concept */
#include "reactor_concept_vulkan_kernel54.generated.h"
#include <string.h>

/* m1.01-m1.10: package prometheus.core@1 / kernel-54-default; binding 0 AS/read, binding 1 storage/write; dispatch 1,1,1; synchronous completion establishes host visibility for coherent evidence. */
int prom_concept_vulkan_kernel54_execute(const prom_vk_runtime_services* services, prom_shader_package* package, VkAccelerationStructureKHR admitted_tlas, uint32_t* out_evidence) {
  prom_vk_buffer evidence = {0}; VkDescriptorSetLayoutBinding bindings[2] = {0}; VkDescriptorSetLayout layout = VK_NULL_HANDLE; VkPipelineLayout pipeline_layout = VK_NULL_HANDLE; VkDescriptorPool pool = VK_NULL_HANDLE; VkDescriptorSet set = VK_NULL_HANDLE; VkPipeline pipeline = VK_NULL_HANDLE; VkShaderModule module = VK_NULL_HANDLE; VkCommandBuffer command = VK_NULL_HANDLE; VkFence fence = VK_NULL_HANDLE; const char* entry = NULL; prom_shader_package_diagnostic diagnostic; VkResult result; uint32_t zero = 0u;
  VkDescriptorSetLayoutCreateInfo layout_info = {0}; VkPipelineLayoutCreateInfo pipeline_layout_info = {0}; VkDescriptorPoolSize pool_sizes[2] = {0}; VkDescriptorPoolCreateInfo pool_info = {0}; VkDescriptorSetAllocateInfo allocate_info = {0}; VkWriteDescriptorSet writes[2] = {0}; VkWriteDescriptorSetAccelerationStructureKHR as_write = {0}; VkDescriptorBufferInfo evidence_info = {0}; VkPipelineShaderStageCreateInfo stage = {0}; VkComputePipelineCreateInfo pipeline_info = {0}; VkCommandBufferAllocateInfo command_allocate = {0}; VkCommandBufferBeginInfo command_begin = {0}; VkFenceCreateInfo fence_info = {0}; VkSubmitInfo submit_info = {0};
  if (services == NULL || package == NULL || admitted_tlas == VK_NULL_HANDLE || out_evidence == NULL) return PROM_ERROR;
  *out_evidence = 0u;
  result = prom_vk_create_buffer(services->physical_device, services->device, services->test_flags, sizeof(uint32_t), VK_BUFFER_USAGE_STORAGE_BUFFER_BIT, VK_MEMORY_PROPERTY_HOST_VISIBLE_BIT | VK_MEMORY_PROPERTY_HOST_COHERENT_BIT, 1, &evidence); if (result != VK_SUCCESS || evidence.mapped == NULL) goto cleanup;
  memcpy(evidence.mapped, &zero, sizeof(zero));
  bindings[0].binding=0u; bindings[0].descriptorType=VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR; bindings[0].descriptorCount=1u; bindings[0].stageFlags=VK_SHADER_STAGE_COMPUTE_BIT; bindings[1].binding=1u; bindings[1].descriptorType=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; bindings[1].descriptorCount=1u; bindings[1].stageFlags=VK_SHADER_STAGE_COMPUTE_BIT;
  layout_info.sType=VK_STRUCTURE_TYPE_DESCRIPTOR_SET_LAYOUT_CREATE_INFO; layout_info.bindingCount=2u; layout_info.pBindings=bindings; if (vkCreateDescriptorSetLayout(services->device,&layout_info,NULL,&layout)!=VK_SUCCESS) goto cleanup;
  pipeline_layout_info.sType=VK_STRUCTURE_TYPE_PIPELINE_LAYOUT_CREATE_INFO; pipeline_layout_info.setLayoutCount=1u; pipeline_layout_info.pSetLayouts=&layout; if (vkCreatePipelineLayout(services->device,&pipeline_layout_info,NULL,&pipeline_layout)!=VK_SUCCESS) goto cleanup;
  pool_sizes[0].type=VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR; pool_sizes[0].descriptorCount=1u; pool_sizes[1].type=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; pool_sizes[1].descriptorCount=1u; pool_info.sType=VK_STRUCTURE_TYPE_DESCRIPTOR_POOL_CREATE_INFO; pool_info.maxSets=1u; pool_info.poolSizeCount=2u; pool_info.pPoolSizes=pool_sizes; if (vkCreateDescriptorPool(services->device,&pool_info,NULL,&pool)!=VK_SUCCESS) goto cleanup;
  allocate_info.sType=VK_STRUCTURE_TYPE_DESCRIPTOR_SET_ALLOCATE_INFO; allocate_info.descriptorPool=pool; allocate_info.descriptorSetCount=1u; allocate_info.pSetLayouts=&layout; if (vkAllocateDescriptorSets(services->device,&allocate_info,&set)!=VK_SUCCESS) goto cleanup;
  as_write.sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET_ACCELERATION_STRUCTURE_KHR; as_write.accelerationStructureCount=1u; as_write.pAccelerationStructures=&admitted_tlas; evidence_info.buffer=evidence.buffer; evidence_info.range=sizeof(uint32_t); writes[0].sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[0].pNext=&as_write; writes[0].dstSet=set; writes[0].dstBinding=0u; writes[0].descriptorCount=1u; writes[0].descriptorType=VK_DESCRIPTOR_TYPE_ACCELERATION_STRUCTURE_KHR; writes[1].sType=VK_STRUCTURE_TYPE_WRITE_DESCRIPTOR_SET; writes[1].dstSet=set; writes[1].dstBinding=1u; writes[1].descriptorCount=1u; writes[1].descriptorType=VK_DESCRIPTOR_TYPE_STORAGE_BUFFER; writes[1].pBufferInfo=&evidence_info; vkUpdateDescriptorSets(services->device,2u,writes,0u,NULL);
  if (!prom_shader_package_create_module(package,services->device,"kernel-54-default",&module,&entry,&diagnostic)) goto cleanup;
  stage.sType=VK_STRUCTURE_TYPE_PIPELINE_SHADER_STAGE_CREATE_INFO; stage.stage=VK_SHADER_STAGE_COMPUTE_BIT; stage.module=module; stage.pName=entry; pipeline_info.sType=VK_STRUCTURE_TYPE_COMPUTE_PIPELINE_CREATE_INFO; pipeline_info.stage=stage; pipeline_info.layout=pipeline_layout; if (vkCreateComputePipelines(services->device,VK_NULL_HANDLE,1u,&pipeline_info,NULL,&pipeline)!=VK_SUCCESS) goto cleanup;
  command_allocate.sType=VK_STRUCTURE_TYPE_COMMAND_BUFFER_ALLOCATE_INFO; command_allocate.commandPool=services->compute_command_pool; command_allocate.level=VK_COMMAND_BUFFER_LEVEL_PRIMARY; command_allocate.commandBufferCount=1u; if (vkAllocateCommandBuffers(services->device,&command_allocate,&command)!=VK_SUCCESS) goto cleanup; command_begin.sType=VK_STRUCTURE_TYPE_COMMAND_BUFFER_BEGIN_INFO; command_begin.flags=VK_COMMAND_BUFFER_USAGE_ONE_TIME_SUBMIT_BIT; if (vkBeginCommandBuffer(command,&command_begin)!=VK_SUCCESS) goto cleanup; vkCmdBindPipeline(command,VK_PIPELINE_BIND_POINT_COMPUTE,pipeline); vkCmdBindDescriptorSets(command,VK_PIPELINE_BIND_POINT_COMPUTE,pipeline_layout,0u,1u,&set,0u,NULL); vkCmdDispatch(command,1u,1u,1u); if (vkEndCommandBuffer(command)!=VK_SUCCESS) goto cleanup;
  fence_info.sType=VK_STRUCTURE_TYPE_FENCE_CREATE_INFO; if (vkCreateFence(services->device,&fence_info,NULL,&fence)!=VK_SUCCESS) goto cleanup; submit_info.sType=VK_STRUCTURE_TYPE_SUBMIT_INFO; submit_info.commandBufferCount=1u; submit_info.pCommandBuffers=&command; if (vkQueueSubmit(services->compute_queue,1u,&submit_info,fence)!=VK_SUCCESS || vkWaitForFences(services->device,1u,&fence,VK_TRUE,UINT64_MAX)!=VK_SUCCESS) goto cleanup; memcpy(out_evidence,evidence.mapped,sizeof(*out_evidence)); result=VK_SUCCESS;
cleanup:
  if (fence != VK_NULL_HANDLE) { vkDestroyFence(services->device,fence,NULL); }
  if (command != VK_NULL_HANDLE) { vkFreeCommandBuffers(services->device,services->compute_command_pool,1u,&command); }
  if (pipeline != VK_NULL_HANDLE) { vkDestroyPipeline(services->device,pipeline,NULL); }
  if (module != VK_NULL_HANDLE) { vkDestroyShaderModule(services->device,module,NULL); }
  if (pool != VK_NULL_HANDLE) { vkDestroyDescriptorPool(services->device,pool,NULL); }
  if (pipeline_layout != VK_NULL_HANDLE) { vkDestroyPipelineLayout(services->device,pipeline_layout,NULL); }
  if (layout != VK_NULL_HANDLE) { vkDestroyDescriptorSetLayout(services->device,layout,NULL); }
  prom_vk_destroy_buffer(services->device,&evidence); return result == VK_SUCCESS ? PROM_OK : PROM_ERROR;
}
`)
	mapDoc := map[string]any{"schema": "concept-vulkan-source-map.v1", "source": "Examples/Concept-Vulkan/kernel54_probe.concept", "function": p.Function, "mir": m.Operations, "generated_symbol": "prom_concept_vulkan_kernel54_execute", "generated_c_lines": map[string]int{"start": 6, "end": 31}}
	mapJSON, err := json.MarshalIndent(mapDoc, "", "  ")
	if err != nil {
		return nil, err
	}
	mapJSON = append(mapJSON, '\n')
	manifest := map[string]any{"schema": "concept-vulkan-generation-manifest.v1", "profile": "Vulkan", "compiler": CompilerID, "source": "Examples/Concept-Vulkan/kernel54_probe.concept", "source_sha256": digest(source), "package": "prometheus.core@1", "entry": "kernel-54-default", "descriptor_contract": "binding 0 acceleration_structure read; binding 1 storage_buffer write", "target": "Prometheus Vulkan kernel-54 M1", "options": map[string]string{"paths": "repository-relative", "timestamps": "forbidden"}, "files": []map[string]string{{"path": "reactor_concept_vulkan_kernel54.generated.c", "sha256": digest(c)}, {"path": "reactor_concept_vulkan_kernel54.generated.h", "sha256": digest(h)}, {"path": "reactor_concept_vulkan_kernel54.mir.json", "sha256": digest(mir)}, {"path": "reactor_concept_vulkan_kernel54.map.json", "sha256": digest(mapJSON)}}}
	manifestJSON, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	manifestJSON = append(manifestJSON, '\n')
	return Outputs{"reactor_concept_vulkan_kernel54.generated.c": c, "reactor_concept_vulkan_kernel54.generated.h": h, "reactor_concept_vulkan_kernel54.mir.json": mir, "reactor_concept_vulkan_kernel54.map.json": mapJSON, "reactor_concept_vulkan_kernel54.manifest.json": manifestJSON}, nil
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func Write(dir string, out Outputs) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := os.WriteFile(filepath.Join(dir, k), out[k], 0644); err != nil {
			return err
		}
	}
	return nil
}
func Check(dir string, out Outputs) error {
	for n, want := range out {
		got, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			return fmt.Errorf("CV3001 stale generated output %s: %w", n, err)
		}
		if string(got) != string(want) {
			return fmt.Errorf("CV3001 stale or hand-edited generated output %s", n)
		}
	}
	return nil
}

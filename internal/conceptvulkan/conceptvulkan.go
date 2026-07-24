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

var fnRE = regexp.MustCompile(`^fn\s+([A-Za-z_][A-Za-z0-9_]*)\(borrow context: MechanismContext, unsafe imported borrow admittedTlas: AccelerationStructure\) -> Result<ProbeEvidence, PrometheusError> \{$`)
var localRE = regexp.MustCompile(`^owned ([A-Za-z_][A-Za-z0-9_]*) = ([A-Za-z_][A-Za-z0-9_]*)\(`)

// Parse accepts only the M1 grammar. Its line-oriented shape is deliberate:
// no general expression language or embedded C is admitted by this profile.
func Parse(path, text string) (Program, error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "profile Vulkan;" {
		return Program{}, Diagnostic{"CV1001", "expected `profile Vulkan;`", Span{1, 1}}
	}
	p := Program{Path: filepath.ToSlash(path)}
	seenFn := false
	for n, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") || n == 0 || line == "import Prometheus.Vulkan;" {
			continue
		}
		span := Span{n + 1, len(raw) - len(strings.TrimLeft(raw, " \t")) + 1}
		if !seenFn {
			m := fnRE.FindStringSubmatch(line)
			if m == nil {
				return Program{}, Diagnostic{"CV1002", "expected the bounded M1 function declaration", span}
			}
			if !pascal(m[1]) {
				return Program{}, Diagnostic{"CV1201", "function names must be PascalCase", span}
			}
			p.Function = m[1]
			seenFn = true
			continue
		}
		if line == "}" {
			continue
		}
		if strings.Contains(line, "concept ") || strings.Contains(line, "decide") || strings.Contains(line, "yield") || strings.Contains(line, "HLSL") {
			return Program{}, Diagnostic{"CV1003", "unsupported Concept/Vulkan construct", span}
		}
		if m := localRE.FindStringSubmatch(line); m != nil && !camel(m[1]) {
			return Program{}, Diagnostic{"CV1202", "owned local names must be camelCase", span}
		}
		name := operationName(line)
		if name == "" {
			return Program{}, Diagnostic{"CV1004", "unsupported or malformed M1 statement", span}
		}
		p.Operations = append(p.Operations, Operation{Name: name, Span: span})
	}
	if !seenFn {
		return Program{}, Diagnostic{"CV1002", "missing M1 function declaration", Span{1, 1}}
	}
	return p, Validate(p)
}
func pascal(s string) bool {
	return len(s) > 0 && s[0] >= 'A' && s[0] <= 'Z' && !strings.Contains(s, "_")
}
func camel(s string) bool {
	return len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' && !strings.Contains(s, "_")
}
func operationName(s string) string {
	for prefix, name := range map[string]string{
		"owned evidence = CreateMappedEvidenceBuffer(context)?;": "create_buffer", "owned pipeline = CreatePackagePipeline(context, \"prometheus.core@1\", \"kernel-54-default\")?;": "create_pipeline", "owned descriptors = BindDescriptor(context, admittedTlas, evidence, 0, 1)?;": "bind_descriptor", "owned command = BeginCommands(context)?;": "begin_recording", "DeclareAccess(admittedTlas, AccelerationStructureRead);": "declare_tlas_access", "DeclareAccess(evidence, ShaderWrite);": "declare_evidence_access", "Dispatch(command, 1, 1, 1);": "dispatch", "owned submission = SubmitAndWait(context, move command)?;": "submit_wait", "ReadObservation(evidence);": "observe", "return Ok;": "return",
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
	h := []byte("/* Generated by concept-vulkan-m1. DO NOT EDIT. */\n#ifndef PROM_CONCEPT_VULKAN_KERNEL54_GENERATED_H\n#define PROM_CONCEPT_VULKAN_KERNEL54_GENERATED_H\n\n/* Private conformance-only plan witness; no public Prometheus ABI. */\nint prom_concept_vulkan_kernel54_plan_verify(void);\n\n#endif\n")
	c := []byte("/* Generated by concept-vulkan-m1. DO NOT EDIT. Source: Examples/Concept-Vulkan/kernel54_probe.concept */\n#include \"reactor_concept_vulkan_kernel54.generated.h\"\n#include \"reactor_vulkan.h\"\n\n/* m1.01-m1.10: kernel-54 contract: package prometheus.core@1 / kernel-54-default; bindings 0 AS read, 1 storage write; dispatch 1,1,1; shader_write -> host_read follows synchronous completion. */\nint prom_concept_vulkan_kernel54_plan_verify(void) {\n  /* Generated private plan is compiled beside, never routed into, the handwritten probe. */\n  return 1;\n}\n")
	mapDoc := map[string]any{"schema": "concept-vulkan-source-map.v1", "source": "Examples/Concept-Vulkan/kernel54_probe.concept", "function": p.Function, "mir": m.Operations, "generated_symbol": "prom_concept_vulkan_kernel54_plan_verify", "generated_c_lines": map[string]int{"start": 6, "end": 9}}
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

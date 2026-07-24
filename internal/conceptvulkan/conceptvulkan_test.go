package conceptvulkan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const canonical = `profile Vulkan;
import Prometheus.Vulkan;

Result<ProbeEvidence, PrometheusError> Execute(
    borrow MechanismContext context,
    unsafe imported borrow AccelerationStructure admittedTlas)
{
    owned MappedEvidenceBuffer evidence =
        CreateMappedEvidenceBuffer(context)?;
    owned ComputePipeline pipeline =
        CreatePackagePipeline(
            context,
            "prometheus.core@1",
            "kernel-54-default")?;
    owned DescriptorSet descriptors =
        BindDescriptor(context, admittedTlas, evidence, 0, 1)?;
    owned CommandRecording command =
        BeginCommands(context)?;
    DeclareAccess(admittedTlas, AccelerationStructureRead);
    DeclareAccess(evidence, ShaderWrite);
    Dispatch(command, 1, 1, 1);
    owned Submission submission =
        SubmitAndWait(context, move command)?;
    ReadObservation(evidence);
    return Ok;
}
`

func TestCanonicalParseLowerAndGenerateAreDeterministic(t *testing.T) {
	p, err := Parse("Examples/Concept-Vulkan/kernel54_probe.concept", canonical)
	if err != nil {
		t.Fatal(err)
	}
	if p.Function != "Execute" || len(p.Operations) != 10 || p.Operations[0].Span.Line != 8 {
		t.Fatalf("unexpected parsed program: %#v", p)
	}
	a, err := Generate(p, []byte(canonical))
	if err != nil {
		t.Fatal(err)
	}
	b, err := Generate(p, []byte(canonical))
	if err != nil {
		t.Fatal(err)
	}
	for n := range a {
		if string(a[n]) != string(b[n]) {
			t.Fatalf("nondeterministic %s", n)
		}
	}
	if !strings.Contains(MIRText(Lower(p)), "vkCmdDispatch(1, 1, 1)") {
		t.Fatal("MIR omitted dispatch mapping")
	}
	for _, n := range []string{"reactor_concept_vulkan_kernel54.map.json", "reactor_concept_vulkan_kernel54.manifest.json"} {
		var v any
		if err := json.Unmarshal(a[n], &v); err != nil {
			t.Fatalf("%s: %v", n, err)
		}
	}
}

func TestDiagnosticsAreStableAndLocated(t *testing.T) {
	cases := []struct{ name, src, code string }{
		{"profile", strings.Replace(canonical, "profile Vulkan;", "profile Metal;", 1), "CV1001"},
		{"function naming", strings.Replace(canonical, " Execute(", " execute(", 1), "CV1201"},
		{"local naming", strings.Replace(canonical, "MappedEvidenceBuffer evidence", "MappedEvidenceBuffer evidence_buffer", 1), "CV1202"},
		{"dispatch", strings.Replace(canonical, "Dispatch(command, 1, 1, 1);", "Dispatch(command, 2, 1, 1);", 1), "CV1004"},
		{"unsupported", strings.Replace(canonical, "return Ok;", "yield;", 1), "CV1003"},
		{"alien fn", "profile Vulkan;\nfn Execute() {}\n", "CV1005"},
		{"alien parameter", "profile Vulkan;\nResult Execute(context: MechanismContext) {}\n", "CV1005"},
		{"alien arrow", "profile Vulkan;\nResult Execute() -> ProbeEvidence {}\n", "CV1005"},
		{"alien let", strings.Replace(canonical, "owned MappedEvidenceBuffer evidence =\n        CreateMappedEvidenceBuffer(context)?;", "let evidence = CreateMappedEvidenceBuffer(context)?;", 1), "CV1005"},
		{"alien var", strings.Replace(canonical, "owned MappedEvidenceBuffer evidence =\n        CreateMappedEvidenceBuffer(context)?;", "var evidence = CreateMappedEvidenceBuffer(context)?;", 1), "CV1005"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("test.concept", tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err=%v want %s", err, tc.code)
			}
		})
	}
}

func TestLexerRetainsSourceSpans(t *testing.T) {
	tokens, err := Lex("profile Vulkan;\nResult Probe;\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(tokens) != 6 || tokens[3].Lexeme != "Result" || tokens[3].Span != (Span{Line: 2, Column: 1}) {
		t.Fatalf("tokens = %#v", tokens)
	}
}

func TestCheckRejectsHandEdit(t *testing.T) {
	p, err := Parse("x.concept", canonical)
	if err != nil {
		t.Fatal(err)
	}
	o, err := Generate(p, []byte(canonical))
	if err != nil {
		t.Fatal(err)
	}
	d := t.TempDir()
	if err := Write(d, o); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "reactor_concept_vulkan_kernel54.generated.c"), []byte("edited"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Check(d, o); err == nil || !strings.Contains(err.Error(), "CV3001") {
		t.Fatalf("check error=%v", err)
	}
}

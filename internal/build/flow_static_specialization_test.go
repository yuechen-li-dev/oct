//go:build integration

package build

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

const persistentPolicySpecimen = "../../Language/ControlFlow/OctomataUtilityWhen/runtime/valid/persistent_policy_controller.octest"

type generatedFlowMetrics struct {
	Bytes         int
	Lines         int
	Imports       int
	HelperFuncs   int
	FlowFields    int
	UsesReflect   bool
	HasClone      bool
	HasRange      bool
	HasHistory    bool
	HasResume     bool
	HasUtilityMap bool
}

func compilePersistentPolicySpecimen(t *testing.T) string {
	t.Helper()
	program, err := project.LoadForTest(persistentPolicySpecimen)
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	pkg := program.Packages[program.Entry]
	if len(pkg.Flows) != 1 {
		t.Fatalf("expected one specimen flow, got %d", len(pkg.Flows))
	}
	flow, err := lowerFlow(program, program.Entry, pkg.Flows[0], pkg)
	if err != nil {
		t.Fatal(err)
	}
	module := MIRModule{
		EntryPackage: program.Entry,
		Flows:        []MIRFlow{flow},
		Records: []MIRRecord{{
			Package: program.Entry,
			Name:    pkg.Flows[0].Name + "BoardSnapshot",
			Fields:  flow.Board,
		}},
	}
	source, err := emitGoWithOptions(module, goEmitOptions{packageName: "flowbench", includeMain: false})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func measureGeneratedFlow(t *testing.T, source string) generatedFlowMetrics {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "generated.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	metrics := generatedFlowMetrics{
		Bytes:         len(source),
		Lines:         strings.Count(source, "\n"),
		Imports:       len(file.Imports),
		UsesReflect:   strings.Contains(source, "\"reflect\""),
		HasClone:      strings.Contains(source, "func __octClone"),
		HasRange:      strings.Contains(source, "type __octRange struct"),
		HasHistory:    strings.Contains(source, "history []string"),
		HasResume:     strings.Contains(source, "hasResumeTarget bool"),
		HasUtilityMap: strings.Contains(source, "utilitySites map[int]__octUtilitySiteState"),
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if strings.HasPrefix(declaration.Name.Name, "__oct") {
				metrics.HelperFuncs++
			}
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok || typeSpec.Name.Name != "__octFlow_MainValid_PersistentPolicyController" {
					continue
				}
				if structType, ok := typeSpec.Type.(*ast.StructType); ok {
					metrics.FlowFields = len(structType.Fields.List)
				}
			}
		}
	}
	return metrics
}

func TestPersistentPolicyFlowGeneratedStructure(t *testing.T) {
	source := compilePersistentPolicySpecimen(t)
	metrics := measureGeneratedFlow(t, source)
	t.Logf("generated flow metrics: %+v", metrics)
	for name, present := range map[string]bool{
		"reflect import":       metrics.UsesReflect,
		"generic clone helper": metrics.HasClone,
		"range support":        metrics.HasRange,
		"history storage":      metrics.HasHistory,
		"resume storage":       metrics.HasResume,
		"generic utility map":  metrics.HasUtilityMap,
	} {
		if present {
			t.Errorf("specialized scalar policy flow retained unused %s", name)
		}
	}
	if !strings.Contains(source, "utilitySite0 __octScalarUtilitySiteState[int]") || !strings.Contains(source, "__octUtilSelectScalar[int](&f.utilitySite0") {
		t.Fatalf("scalar policy site was not lowered to typed persistent state:\n%s", source)
	}
	if metrics.FlowFields > 8 {
		t.Fatalf("specialized flow retained %d fields; want at most 8", metrics.FlowFields)
	}
	if repeated := compilePersistentPolicySpecimen(t); repeated != source {
		t.Fatal("repeated generation was not deterministic")
	}
}

func TestAdvancedFlowFeaturesRetainRequiredStorage(t *testing.T) {
	flow := MIRFlow{
		Package:    "Main",
		Name:       "Advanced",
		Return:     "Int",
		EntryState: "Start",
		States: []MIRFlowState{{
			Name:       "Start",
			Statements: []MIRFlowStmt{MIRFlowRemember{}, MIRFlowResume{}},
		}},
	}
	features := analyzeFlowFeatures(flow, map[string]bool{"StateHistory": true})
	if !features.NeedsHistory || !features.NeedsResume {
		t.Fatalf("advanced flow analysis dropped required storage: %+v", features)
	}
	var source strings.Builder
	if err := emitGoFlow(&source, flow, features); err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"history []string", "hasResumeTarget bool", "resumeTarget int", "f.history = append"} {
		if !strings.Contains(source.String(), required) {
			t.Errorf("advanced flow emission omitted %q", required)
		}
	}
}

func TestPersistentPolicyFlowGeneratedGoCompiles(t *testing.T) {
	source := compilePersistentPolicySpecimen(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "generated.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module flowbench\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "./...")
	command.Dir = dir
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("generated Go did not compile: %v\n%s", err, output)
	}
}

func TestPersistentPolicyFlowBenchmark(t *testing.T) {
	if os.Getenv("OCT_FLOW_SPECIALIZATION_BENCH") != "1" {
		t.Skip("set OCT_FLOW_SPECIALIZATION_BENCH=1 to run generated-vs-handwritten benchmarks")
	}
	source := compilePersistentPolicySpecimen(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "generated.go"), []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module flowbench\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "flow_benchmark_test.go"), []byte(flowBenchmarkHarness), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("go", "test", "-run", "^$", "-bench", "Benchmark", "-benchmem", "-count=5")
	command.Dir = dir
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("benchmark generated Go: %v\n%s", err, output)
	}
	t.Logf("generated-vs-handwritten benchmark:\n%s", output)
}

const flowBenchmarkHarness = `package flowbench

import "testing"

var benchmarkChoice int

func BenchmarkCompiledPersistentPolicy(b *testing.B) {
	controller := fn_MainValid_PersistentPolicyController().(*__octFlow_MainValid_PersistentPolicyController)
	controller.__octStep()
	controller.__octStep()
	controller.__octStep()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		controller.__octStep()
	}
	benchmarkChoice = controller.board.LastChoice
}

type handwrittenPolicyController struct {
	tick int
	choice int
	score int
	commitAge int
}

func (controller *handwrittenPolicyController) step() int {
	nextChoice := 1
	nextScore := 60
	if controller.tick == 1 && 65 > nextScore {
		nextChoice = 2
		nextScore = 65
	}
	if controller.tick >= 2 && 90 > nextScore {
		nextChoice = 2
		nextScore = 90
	}
	if controller.commitAge > 0 {
		currentValid := controller.choice == 1 || controller.choice == 2
		if currentValid && (controller.commitAge < 3 || nextScore <= controller.score+8) {
			nextChoice = controller.choice
			nextScore = controller.score
		}
	}
	if controller.commitAge == 0 || controller.choice != nextChoice {
		controller.choice = nextChoice
		controller.score = nextScore
		controller.commitAge = 1
	} else {
		controller.score = nextScore
		controller.commitAge++
	}
	controller.tick++
	return controller.choice
}

func BenchmarkHandwrittenPersistentPolicy(b *testing.B) {
	controller := handwrittenPolicyController{}
	controller.step()
	controller.step()
	controller.step()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkChoice = controller.step()
	}
}
`

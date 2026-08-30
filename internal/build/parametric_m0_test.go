package build

import (
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// This is backend-boundary validation. User-visible parametric semantics live
// in Language/Types/ParametricsM0; this test audits that their real lowering
// product is ordinary concrete FLOW/Go with no template runtime.
func TestParametricM0ErasesBeforeOrdinaryFlowAndGoLowering(t *testing.T) {
	program, err := project.LoadForTest("../../Language/Types/ParametricsM0/valid/ParametricQueryValid/parametric_query_and_helper.octest")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	if program.Parametrics.FunctionInstantiations != 2 || program.Parametrics.RecordInstantiations != 2 || program.Parametrics.FlowInstantiations != 2 {
		t.Fatalf("identical applications were not deduplicated: %#v", program.Parametrics)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}

	flows := map[string]bool{
		"Filtered__Job":           false,
		"Filtered__InventoryItem": false,
	}
	for _, flow := range module.Flows {
		if _, ok := flows[flow.Name]; !ok {
			continue
		}
		flows[flow.Name] = true
		if len(flow.States) != 1 || flow.States[0].Name != "Scan" {
			t.Fatalf("parametric query %s did not reuse the ordinary one-state Query-M0 FLOW shape", flow.Name)
		}
	}
	for name, found := range flows {
		if !found {
			t.Fatalf("missing concrete %s FLOW specialization", name)
		}
	}

	generated, err := emitGo(module)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"Oct template provenance:",
		"type ParametricQueryValid_MaterializedFilter__Job struct",
		"type ParametricQueryValid_MaterializedFilter__InventoryItem struct",
		"type __octFlow_ParametricQueryValid_Filtered__Job struct",
		"fn_ParametricQueryValid_FirstWhere__Job",
		"fn_ParametricQueryValid_FirstWhere__InventoryItem",
	} {
		if !strings.Contains(generated, required) {
			t.Fatalf("generated Go missing concrete specialization %q", required)
		}
	}
	for _, forbidden := range []string{"TemplateRuntime", "GenericDictionary", "TypeArguments"} {
		if strings.Contains(generated, forbidden) {
			t.Fatalf("generated Go retained forbidden parametric runtime shape %q", forbidden)
		}
	}

	selectorProgram, err := project.LoadForTest("../../Language/Types/ParametricsM0/valid/ParametricsM0Valid/parametric_records_functions_selectors.octest")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(selectorProgram); err != nil {
		t.Fatal(err)
	}
	selectorModule, err := lowerProgram(selectorProgram, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	selectorSubjects := map[string]string{}
	for _, selector := range selectorModule.Selectors {
		selectorSubjects[selector.Field.Name] = string(selector.Field.Subject.Kind) + ":" + selector.Field.Subject.Identity
	}
	if selectorSubjects["ID"] != "nominal-record:ParametricsM0Valid.Job" || selectorSubjects["SKU"] != "nominal-record:ParametricsM0Valid.InventoryItem" {
		t.Fatalf("selector FieldRef subjects crossed nominal instantiations: %#v", selectorSubjects)
	}
	selectorGo, err := emitGo(selectorModule)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{"fn_ParametricsM0Valid___oct_selector__Job__ID", "fn_ParametricsM0Valid___oct_selector__InventoryItem__SKU", ".ID", ".SKU"} {
		if !strings.Contains(selectorGo, required) {
			t.Fatalf("generated selector Go missing direct concrete shape %q", required)
		}
	}
}

func TestTemplateTortureM0ErasesAndEmitsDeterministicGo(t *testing.T) {
	program, err := project.LoadForTest("../../Language/Types/TemplateTortureM0/valid/TemplateTortureValid")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	generated, err := emitGo(module)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(generated, "\"math\"") || !strings.Contains(generated, "__octComplexReal") {
		t.Fatal("Complex helper emission must carry its math import")
	}
	for _, forbidden := range []string{"TemplateRuntime", "TypeParameters", "runtime type dictionary", "reflect.TypeOf((*T)"} {
		if strings.Contains(generated, forbidden) {
			t.Fatalf("generated Go retained forbidden template runtime shape %q", forbidden)
		}
	}
	stats := program.Parametrics
	t.Logf("template_torture_m0 templates=%d records=%d functions=%d flows=%d requests=%d reuse=%d max_depth=%d elaboration_ns=%d generated_declarations=%d generated_go_bytes=%d",
		stats.TemplateRecords+stats.TemplateFunctions+stats.TemplateFlows,
		stats.RecordInstantiations,
		stats.FunctionInstantiations,
		stats.FlowInstantiations,
		stats.InstantiationRequests,
		stats.ReusedInstantiations,
		stats.MaximumSpecializationDepth,
		stats.ElaborationNanoseconds,
		len(module.Records)+len(module.Functions)+len(module.Flows),
		len(generated),
	)
}

func TestParametricM0QueryMIRMatchesHandwrittenConcreteShape(t *testing.T) {
	program, err := project.Load("../../docs/internal/evidence/OCT_PARAMETRICS_M0/benchmarks/query_runtime.oct")
	if err != nil {
		t.Fatal(err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		t.Fatal(err)
	}
	module, err := lowerProgram(program, compileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	var parametric, handwritten *MIRFlow
	for i := range module.Flows {
		switch module.Flows[i].Name {
		case "ParametricFilter__Job":
			parametric = &module.Flows[i]
		case "HandwrittenFilter":
			handwritten = &module.Flows[i]
		}
	}
	if parametric == nil || handwritten == nil {
		t.Fatalf("missing comparison flows: %#v", module.Flows)
	}
	parametricCopy, handwrittenCopy := *parametric, *handwritten
	parametricCopy.Name, handwrittenCopy.Name = "Filter", "Filter"
	if !reflect.DeepEqual(parametricCopy, handwrittenCopy) {
		t.Fatalf("parametric query MIR differs from handwritten concrete query\nparametric: %#v\nhandwritten: %#v", parametricCopy, handwrittenCopy)
	}
}

package build

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

// User-visible catalog behavior lives in Libraries/DatabaseTemplates. This
// test audits only the compiler boundary: authored templates have disappeared,
// provenance remains inspectable, and generated execution stays concrete.
func TestDatabaseTemplatesM0GeneratedBoundary(t *testing.T) {
	program, err := project.LoadForTest("../../Libraries/DatabaseTemplates/DatabaseTemplates.Core.octest")
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
	if len(module.Templates) < 10 {
		t.Fatalf("expected composed concrete specializations, got %d: %+v", len(module.Templates), module.Templates)
	}
	t.Logf("parametrics=%+v templates_in_mir=%d selectors=%d", program.Parametrics, len(module.Templates), len(module.Selectors))
	var templated, bespoke *MIRFlow
	for i := range module.Flows {
		switch module.Flows[i].Name {
		case "FilteredView__Job":
			templated = &module.Flows[i]
		case "BespokeFilteredView":
			bespoke = &module.Flows[i]
		}
	}
	if templated == nil || bespoke == nil {
		t.Fatalf("missing template/bespoke FLOW controls: %+v", module.Flows)
	}
	templateCopy, bespokeCopy := *templated, *bespoke
	templateCopy.Name, bespokeCopy.Name = "FilteredView", "FilteredView"
	if !reflect.DeepEqual(templateCopy, bespokeCopy) {
		t.Fatalf("template-authored FLOW MIR differs from the equivalent concrete query\ntemplate=%+v\nbespoke=%+v", templateCopy, bespokeCopy)
	}
	emitNormalizedFlowBoundary := func(flow MIRFlow) string {
		var emitted strings.Builder
		features := analyzeFlowFeatures(flow, map[string]bool{})
		if err := emitGoFlow(&emitted, flow, features); err != nil {
			t.Fatal(err)
		}
		if err := emitGoFlowHostFacade(&emitted, flow, features, "NormalizedFilteredView", module.Refinements); err != nil {
			t.Fatal(err)
		}
		normalized := strings.ReplaceAll(emitted.String(), flow.Name, "NormalizedFilteredView")
		// Checkpoint fingerprints deliberately include nominal package/FLOW identity.
		// Normalize that compatibility boundary only after MIR identity was checked above.
		return strings.ReplaceAll(normalized, compiledFlowFingerprint(flow, features), "NORMALIZED_FLOW_FINGERPRINT")
	}
	if templateGo, bespokeGo := emitNormalizedFlowBoundary(*templated), emitNormalizedFlowBoundary(*bespoke); templateGo != bespokeGo {
		t.Fatalf("normalized template and bespoke Go emission differ\ntemplate:\n%s\nbespoke:\n%s", templateGo, bespokeGo)
	}
	selectorSubjects := map[string]string{}
	for _, selector := range module.Selectors {
		selectorSubjects[selector.Field.Name] = string(selector.Field.Subject.Kind) + ":" + selector.Field.Subject.Identity
	}
	if selectorSubjects["ID"] != "nominal-record:DatabaseTemplates.Job" || selectorSubjects["SKU"] != "nominal-record:DatabaseTemplates.InventoryItem" || selectorSubjects["Status"] != "nominal-record:DatabaseTemplates.Job" {
		t.Fatalf("catalog selector subjects lost nominal provenance: %+v", selectorSubjects)
	}
	generated, err := emitGo(module)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"Oct template provenance:",
		"DatabaseTemplates.MaterializedFilter<DatabaseTemplates.Job>",
		"Oct selector provenance:",
		"Oct template override: DatabaseTemplates.BoundedExtent<DatabaseTemplates.Job> fields=MaxRecords",
		"type DatabaseTemplates_JobQueue__Job__String__Int struct",
		"type __octFlow_DatabaseTemplates_FilteredView__InventoryItem struct",
	} {
		if !strings.Contains(generated, required) {
			t.Fatalf("generated Go missing catalog boundary evidence %q; templates=%+v", required, module.Templates)
		}
	}
	for _, forbidden := range []string{"TemplateRuntime", "GenericDictionary", "TemplateRegistry"} {
		if strings.Contains(generated, forbidden) {
			t.Fatalf("generated Go retained forbidden template machinery %q", forbidden)
		}
	}
	for i := 0; i < 5; i++ {
		reloaded, err := project.LoadForTest("../../Libraries/DatabaseTemplates/DatabaseTemplates.Core.octest")
		if err != nil {
			t.Fatal(err)
		}
		if err := typecheck.CheckProgram(reloaded); err != nil {
			t.Fatal(err)
		}
		reloadedModule, err := lowerProgram(reloaded, compileOptions{})
		if err != nil {
			t.Fatal(err)
		}
		reloadedGenerated, err := emitGo(reloadedModule)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal([]byte(generated), []byte(reloadedGenerated)) {
			t.Fatalf("generated template artifact changed across identical load/lower run %d", i+1)
		}
	}
}

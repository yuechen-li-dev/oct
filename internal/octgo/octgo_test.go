package octgo

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/interpret"
	"github.com/yuechen-li-dev/oct/internal/project"
)

func TestLoadPackageProducesDeterministicBoundedModel(t *testing.T) {
	directory := specimenDirectory(t)
	first, err := LoadPackage(directory)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadPackage(directory)
	if err != nil {
		t.Fatal(err)
	}
	if first.Package.Path != "github.com/yuechen-li-dev/oct/experimental/octgo/specimen" || first.Package.Name != "specimen" {
		t.Fatalf("unexpected identity: %+v", first.Package)
	}
	if len(first.Types) != 1 || first.Types[0].Name != "Threshold" || first.Types[0].Underlying.Kind != "Int" {
		t.Fatalf("unexpected type model: %+v", first.Types)
	}
	if len(first.Constants) != 1 || first.Constants[0].Name != "DefaultThreshold" || first.Constants[0].OctLiteral != "2" {
		t.Fatalf("unexpected constant model: %+v", first.Constants)
	}
	if len(first.Functions) != 2 || first.Functions[0].Name != "Residual" || first.Functions[1].Name != "StrictlyAbove" {
		t.Fatalf("functions are not canonical: %+v", first.Functions)
	}
	if first.Functions[0].Signature != second.Functions[0].Signature || first.Constants[0] != second.Constants[0] {
		t.Fatalf("repeated semantic loads differ: first=%+v second=%+v", first, second)
	}
}

func TestCheckValidatesConceptSignatureAndFreshBridge(t *testing.T) {
	report, err := Check(specimenDirectory(t), false)
	if err != nil {
		t.Fatal(err)
	}
	if report.SelectedConcepts != 1 || report.SelectedFunctions != 2 || report.ConstantWitnesses != 1 || !report.BridgeFresh {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestImportedGoConstantUsesExistingConceptAdmission(t *testing.T) {
	directory := copySpecimen(t)
	replaceFile(t, filepath.Join(directory, "specimen.go"), "const DefaultThreshold Threshold = 2", "const DefaultThreshold Threshold = -1")
	_, err := Check(directory, false)
	if err == nil {
		t.Fatal("expected existing Concept requirement to reject imported constant")
	}
	for _, want := range []string{"Go constant does not satisfy imported Oct Concept", "Threshold", "must be non-negative"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("Concept admission diagnostic missing %q: %v", want, err)
		}
	}
}

func TestSignatureMismatchReportsBothSidesBeforeExecution(t *testing.T) {
	directory := copySpecimen(t)
	path := filepath.Join(directory, "specimen.contracts.oct")
	replaceFile(t, path, "go fn StrictlyAbove(value: Int, threshold: Int) -> Bool", "go fn StrictlyAbove(value: Int, threshold: Float) -> Bool")
	_, err := Check(directory, false)
	if err == nil {
		t.Fatal("expected signature disagreement")
	}
	message := err.Error()
	for _, want := range []string{"Go function", "StrictlyAbove", "func(value int, threshold int) bool", "fn(Int, Float) -> Bool", "parameter 2 is incompatible"} {
		if !strings.Contains(message, want) {
			t.Fatalf("diagnostic missing %q:\n%s", want, message)
		}
	}
}

func TestUnsupportedShapeIsRejectedPrecisely(t *testing.T) {
	directory := copySpecimen(t)
	appendFile(t, filepath.Join(directory, "specimen.go"), "\nfunc Bad(values []int) int { return len(values) }\n")
	contract := filepath.Join(directory, "specimen.contracts.oct")
	appendFile(t, contract, "\ngo fn Bad(values: Int) -> Int\n")
	_, err := Check(directory, true)
	if err == nil || !strings.Contains(err.Error(), "parameter 1: type []int is outside bounded OctGo") {
		t.Fatalf("unexpected unsupported-shape diagnostic: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(directory, "octgo_bridge", "main.go")); !os.IsNotExist(statErr) {
		t.Fatalf("unsupported import partially generated a bridge: %v", statErr)
	}
}

func TestAuthoritativeImportsLowerToCanonicalWrapperMetadata(t *testing.T) {
	c, _, err := loadContract(specimenDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	var imports []ast.FunctionDecl
	for _, fn := range c.companion.Functions {
		if fn.IsGoImport {
			imports = append(imports, fn)
		}
	}
	first := derivedWrapperMetadata(c.model, imports)
	second := derivedWrapperMetadata(c.model, imports)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("wrapper lowering is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if len(first.Functions) != 2 || first.Functions[0].OctName != "Residual" || first.Functions[1].OctName != "StrictlyAbove" {
		t.Fatalf("wrapper functions are not canonical: %+v", first.Functions)
	}
	if first.Functions[1].WireName != "StrictlyAbove" || !reflect.DeepEqual(first.Functions[1].Args, []string{"Int", "Int"}) || first.Functions[1].Return != "Bool" {
		t.Fatalf("wrapper signature was not derived from the import: %+v", first.Functions[1])
	}
}

func TestGeneratedBridgeIsDeterministicAndStaleCheckDoesNotMutate(t *testing.T) {
	directory := copySpecimen(t)
	first, err := Check(directory, true)
	if err != nil {
		t.Fatal(err)
	}
	one, err := os.ReadFile(first.BridgePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Check(directory, true); err != nil {
		t.Fatal(err)
	}
	two, _ := os.ReadFile(first.BridgePath)
	if !bytes.Equal(one, two) {
		t.Fatal("repeated bridge generation is not byte-identical")
	}
	stale := bytes.Replace(one, []byte(`case "Residual":`), []byte(`case "ResidualStale":`), 1)
	if err := os.WriteFile(first.BridgePath, stale, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "bridge is stale") {
		t.Fatalf("expected stale bridge error, got %v", err)
	}
	after, _ := os.ReadFile(first.BridgePath)
	if !bytes.Equal(stale, after) {
		t.Fatal("validation-only check mutated stale output")
	}
}

func TestGoSignatureDriftReportsSemanticMismatchBeforeStaleness(t *testing.T) {
	directory := copySpecimen(t)
	if _, err := Check(directory, true); err != nil {
		t.Fatal(err)
	}
	replaceFile(t, filepath.Join(directory, "specimen.go"), "func StrictlyAbove(value, threshold int) bool {", "func StrictlyAbove(value, threshold, margin int) bool {")
	replaceFile(t, filepath.Join(directory, "specimen.go"), "return value > threshold", "return value > threshold+margin")
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "parameter count differs: Go has 3, Oct expects 2") || strings.Contains(err.Error(), "bridge is stale") {
		t.Fatalf("typed Go signature drift should report the semantic mismatch first, got %v", err)
	}
}

func TestValidCompanionSelectionChangeMakesBridgeStale(t *testing.T) {
	directory := copySpecimen(t)
	if _, err := Check(directory, true); err != nil {
		t.Fatal(err)
	}
	appendFile(t, filepath.Join(directory, "specimen.go"), "\nfunc Below(value, threshold int) bool { return value < threshold }\n")
	appendFile(t, filepath.Join(directory, "specimen.contracts.oct"), "\ngo fn Below(value: Int, threshold: Int) -> Bool\n")
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "bridge is stale") {
		t.Fatalf("valid authoritative declaration change should stale the bridge: %v", err)
	}
}

func TestRemovedGoFunctionReportsMissingIdentity(t *testing.T) {
	directory := copySpecimen(t)
	replaceFile(t, filepath.Join(directory, "specimen.go"), "func StrictlyAbove(value, threshold int) bool {", "func noLongerExported(value, threshold int) bool {")
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "has no exported Go function identity") || strings.Contains(err.Error(), "bridge is stale") {
		t.Fatalf("removed Go function should fail semantic binding before freshness: %v", err)
	}
}

func TestImportedGoCallIsRejectedByCompileTimeRequire(t *testing.T) {
	directory := copySpecimen(t)
	contract := filepath.Join(directory, "specimen.contracts.oct")
	replaceFile(t, contract, `Require(boundary >= 0, "the equality boundary is an admissible threshold")`, `Require(StrictlyAbove(2, 1), "imports are not compile-time operations")`)
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "cannot be evaluated at compile time") {
		t.Fatalf("compile-time Require unexpectedly admitted an imported Go call: %v", err)
	}
}

func TestCheckWithoutOctestStillValidatesCompanion(t *testing.T) {
	directory := copySpecimen(t)
	if err := os.Remove(filepath.Join(directory, "specimen.octest")); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(directory, true); err != nil {
		t.Fatalf("check should not require Octest: %v", err)
	}
}

func TestInterpretedImportedCallHasPreciseCompiledOnlyDiagnostic(t *testing.T) {
	program, err := project.LoadForTest(specimenDirectory(t))
	if err != nil {
		t.Fatal(err)
	}
	err = interpret.ExecuteFunction(program, "Specimen", "EqualityIsExcludedByStrictThreshold", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "OctGo import Specimen.StrictlyAbove is compiled-only") {
		t.Fatalf("unexpected interpreted import behavior: %v", err)
	}
}

func specimenDirectory(t *testing.T) string {
	t.Helper()
	directory, err := filepath.Abs(filepath.Join("..", "..", "experimental", "octgo", "specimen"))
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func copySpecimen(t *testing.T) string {
	t.Helper()
	sourceDir := specimenDirectory(t)
	destination, err := os.MkdirTemp(filepath.Dir(sourceDir), ".octgo-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(destination) })
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(sourceDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(destination, entry.Name()), contents, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifestPath := filepath.Join(destination, "manifest.oct")
	contents, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	newPath := "github.com/yuechen-li-dev/oct/experimental/octgo/" + filepath.Base(destination)
	contents = bytes.ReplaceAll(contents, []byte("github.com/yuechen-li-dev/oct/experimental/octgo/specimen"), []byte(newPath))
	if err := os.WriteFile(manifestPath, contents, 0o644); err != nil {
		t.Fatal(err)
	}
	return destination
}

func replaceFile(t *testing.T, path, old, replacement string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := strings.Replace(string(contents), old, replacement, 1)
	if updated == string(contents) {
		t.Fatalf("replacement target not found in %s: %q", path, old)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func appendFile(t *testing.T, path, suffix string) {
	t.Helper()
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.WriteString(suffix); err != nil {
		t.Fatal(err)
	}
}

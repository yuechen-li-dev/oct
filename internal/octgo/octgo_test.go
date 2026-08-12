package octgo

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	replaceFile(t, path, "fn StrictlyAbove(value: Int, threshold: Int) -> Bool", "fn StrictlyAbove(value: Int, threshold: Float) -> Bool")
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
	appendFile(t, filepath.Join(directory, "specimen.go"), "\nfunc Unsupported(value int) (int, int) { return value, value }\n")
	manifest := filepath.Join(directory, "manifest.oct")
	replaceFile(t, manifest, "Functions: [\n", "Functions: [\n                    WrapperFunction { OctName: \"Unsupported\" WireName: \"Unsupported\" Args: [\"Int\"] Return: \"Int\" Fallible: false },\n")
	contract := filepath.Join(directory, "specimen.contracts.oct")
	appendFile(t, contract, "\nfn Unsupported(value: Int) -> Int { return 0 }\n")
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "multiple results are outside OCTGO-M0") {
		t.Fatalf("unexpected unsupported-shape diagnostic: %v", err)
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

func TestGoSignatureChangeMakesCommittedBridgeStale(t *testing.T) {
	directory := copySpecimen(t)
	if _, err := Check(directory, true); err != nil {
		t.Fatal(err)
	}
	replaceFile(t, filepath.Join(directory, "specimen.go"), "func StrictlyAbove(value, threshold int) bool {", "func StrictlyAbove(value, threshold, margin int) bool {")
	replaceFile(t, filepath.Join(directory, "specimen.go"), "return value > threshold", "return value > threshold+margin")
	replaceFile(t, filepath.Join(directory, "specimen.contracts.oct"), "fn StrictlyAbove(value: Int, threshold: Int) -> Bool", "fn StrictlyAbove(value: Int, threshold: Int, margin: Int) -> Bool")
	replaceFile(t, filepath.Join(directory, "manifest.oct"), `Args: ["Int", "Int"] Return: "Bool"`, `Args: ["Int", "Int", "Int"] Return: "Bool"`)
	_, err := Check(directory, false)
	if err == nil || !strings.Contains(err.Error(), "bridge is stale") {
		t.Fatalf("typed Go signature change should make committed adapter stale, got %v", err)
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

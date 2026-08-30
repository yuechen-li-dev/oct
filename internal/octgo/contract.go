package octgo

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/lex"
	"github.com/yuechen-li-dev/oct/internal/manifestwrapper"
	"github.com/yuechen-li-dev/oct/internal/parse"
	"github.com/yuechen-li-dev/oct/internal/project"
	"github.com/yuechen-li-dev/oct/internal/source"
	"github.com/yuechen-li-dev/oct/internal/typecheck"
)

var ErrNoCompanion = errors.New("no OctGo companion")

type Report struct {
	Package           PackageIdentity
	CompanionPath     string
	BridgePath        string
	TypeCount         int
	ConstantCount     int
	FunctionCount     int
	SelectedConcepts  int
	SelectedFunctions int
	ConstantWitnesses int
	BridgeGenerated   bool
	BridgeFresh       bool
	wrappers          []project.WrapperMetadata
}

type contract struct {
	directory string
	companion ast.File
	path      string
	program   project.Program
	model     PackageModel
	functions []bridgeFunction
	wrapper   project.WrapperMetadata
}

type bridgeFunction struct {
	Go  FunctionModel
	Oct ast.FunctionDecl
}

func Check(path string, generate bool) (Report, error) {
	c, report, err := loadContract(path)
	if err != nil {
		return Report{}, err
	}
	if err := validateContract(&c, &report); err != nil {
		return Report{}, err
	}
	if len(c.functions) > 0 {
		report.wrappers = []project.WrapperMetadata{c.wrapper}
	}
	if len(c.functions) == 0 {
		return report, nil
	}
	contents, err := renderBridge(c.model, c.functions)
	if err != nil {
		return Report{}, err
	}
	report.BridgePath = filepath.Join(c.directory, "octgo_bridge", "main.go")
	if generate {
		if err := writeBridge(report.BridgePath, contents); err != nil {
			return Report{}, err
		}
		report.BridgeGenerated = true
		report.BridgeFresh = true
		return report, nil
	}
	committed, err := os.ReadFile(report.BridgePath)
	if err != nil {
		if os.IsNotExist(err) {
			return Report{}, fmt.Errorf("generated OCTGO bridge is missing: %s\nrun: oct check %s --generate", report.BridgePath, c.directory)
		}
		return Report{}, fmt.Errorf("read generated OCTGO bridge %s: %w", report.BridgePath, err)
	}
	if !bytes.Equal(committed, contents) {
		return Report{}, fmt.Errorf("generated OCTGO bridge is stale: %s\nrun: oct check %s --generate", report.BridgePath, c.directory)
	}
	report.BridgeFresh = true
	return report, nil
}

func loadContract(path string) (contract, Report, error) {
	directory, err := contractDirectory(path)
	if err != nil {
		return contract{}, Report{}, err
	}
	companions, err := filepath.Glob(filepath.Join(directory, "*.contracts.oct"))
	if err != nil {
		return contract{}, Report{}, fmt.Errorf("discover OCTGO companions: %w", err)
	}
	sort.Strings(companions)
	if len(companions) == 0 {
		return contract{}, Report{}, ErrNoCompanion
	}
	if len(companions) != 1 {
		return contract{}, Report{}, fmt.Errorf("OctGo requires exactly one *.contracts.oct companion in %s, found %d", directory, len(companions))
	}
	companion, err := parseCompanion(companions[0])
	if err != nil {
		return contract{}, Report{}, err
	}
	model, err := LoadPackage(directory)
	if err != nil {
		return contract{}, Report{}, err
	}
	program, err := project.Load(directory)
	if err != nil {
		return contract{}, Report{}, fmt.Errorf("load Oct companion project: %w", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return contract{}, Report{}, fmt.Errorf("check Oct companion %s: %w", companions[0], err)
	}
	if companion.Package != program.Entry {
		return contract{}, Report{}, fmt.Errorf("Oct companion package %s does not match selected Oct package %s", companion.Package, program.Entry)
	}
	c := contract{directory: directory, companion: companion, path: companions[0], program: program, model: model}
	report := Report{
		Package: model.Package, CompanionPath: companions[0], TypeCount: len(model.Types),
		ConstantCount: len(model.Constants), FunctionCount: len(model.Functions),
	}
	return c, report, nil
}

func validateContract(c *contract, report *Report) error {
	selectedConcepts := map[string]ast.TypeRef{}
	for _, concept := range c.companion.Concepts {
		goType, ok := c.model.FindType(concept.Name)
		if !ok {
			return fmt.Errorf("Oct concept %s at %s:%d has no Go type identity %s.%s", concept.Name, c.path, concept.Line, c.model.Package.Path, concept.Name)
		}
		if !goType.Supported {
			return fmt.Errorf("Go type %s.%s at %s:%d is outside bounded OctGo: %s", c.model.Package.Path, goType.Name, goType.Position.File, goType.Position.Line, goType.UnsupportedReason)
		}
		if goType.Kind != "scalar" {
			return fmt.Errorf("Go type %s.%s has %s shape, but Oct companion declares scalar concept %s = %s", c.model.Package.Path, goType.Name, goType.Kind, concept.Name, octType(concept.Target))
		}
		if octType(concept.Target) != goType.Underlying.Kind {
			return fmt.Errorf("Go type %s.%s has underlying %s\nOct companion %s expects %s\nerror: imported Go shape does not satisfy companion concept", c.model.Package.Path, goType.Name, goType.Underlying.Kind, c.path, octType(concept.Target))
		}
		selectedConcepts[concept.Name] = concept.Target
		report.SelectedConcepts++
	}
	for _, record := range c.companion.Records {
		if !record.IsConcept {
			continue
		}
		goType, ok := c.model.FindType(record.Name)
		if !ok {
			return fmt.Errorf("Oct record concept %s in %s has no Go type identity %s.%s", record.Name, c.path, c.model.Package.Path, record.Name)
		}
		if !goType.Supported {
			return fmt.Errorf("Go type %s.%s is outside bounded OctGo: %s", c.model.Package.Path, goType.Name, goType.UnsupportedReason)
		}
		if goType.Kind != "struct" || len(goType.Fields) != len(record.Fields) {
			return fmt.Errorf("Go type %s.%s does not match Oct record concept %s: field count/shape differs", c.model.Package.Path, goType.Name, record.Name)
		}
		for index, field := range record.Fields {
			goField := goType.Fields[index]
			if field.Name != goField.Name || octType(field.Type) != goField.Type.Kind {
				return fmt.Errorf("Go type %s.%s field %d is %s %s; Oct companion expects %s %s", c.model.Package.Path, goType.Name, index+1, goField.Name, goField.Type.Kind, field.Name, octType(field.Type))
			}
		}
		selectedConcepts[record.Name] = ast.TypeRef{Name: record.Name}
		report.SelectedConcepts++
	}

	imports := make([]ast.FunctionDecl, 0)
	for _, fn := range c.companion.Functions {
		if !fn.IsGoImport {
			continue
		}
		imports = append(imports, fn)
	}
	sort.Slice(imports, func(i, j int) bool { return imports[i].Name < imports[j].Name })
	for _, octFn := range imports {
		goFn, ok := c.model.FindFunction(octFn.Name)
		if !ok {
			return fmt.Errorf("OctGo import %s in %s has no exported Go function identity %s.%s", octFn.Name, c.path, c.model.Package.Path, octFn.Name)
		}
		if err := validateFunctionSignature(*c, goFn, octFn); err != nil {
			return err
		}
		c.functions = append(c.functions, bridgeFunction{Go: goFn, Oct: octFn})
		report.SelectedFunctions++
	}
	if len(c.functions) > 0 {
		c.wrapper = derivedWrapperMetadata(c.model, imports)
	}
	if len(c.functions) == 0 && len(selectedConcepts) == 0 {
		return fmt.Errorf("%s selects no Go concepts or functions", c.path)
	}
	return validateConstantsWithConcepts(*c, selectedConcepts, report)
}

func validateFunctionSignature(c contract, goFn FunctionModel, octFn ast.FunctionDecl) error {
	for _, parameter := range octFn.Parameters {
		if parameter.Type.Function != nil {
			return fmt.Errorf("OctGo import %s cannot accept function-valued parameter '%s': captured anonymous functions require an environment, and the current OctGo bridge does not transport callable environments", octFn.Name, parameter.Name)
		}
	}
	if octFn.ReturnType.Function != nil {
		return fmt.Errorf("OctGo import %s cannot return a function value: the current OctGo bridge does not transport callable environments", octFn.Name)
	}
	if !goFn.Supported {
		return fmt.Errorf("Go function %s.%s\nhas signature:\n    %s\nerror: %s", c.model.Package.Path, goFn.Name, goFn.Signature, goFn.UnsupportedReason)
	}
	if octFn.IsFallible {
		return fmt.Errorf("Go function %s.%s is non-fallible; OctGo import declarations must also be non-fallible", c.model.Package.Path, goFn.Name)
	}
	if len(goFn.Parameters) != len(octFn.Parameters) {
		return signatureError(c, goFn, octFn, fmt.Sprintf("parameter count differs: Go has %d, Oct expects %d", len(goFn.Parameters), len(octFn.Parameters)))
	}
	for index := range goFn.Parameters {
		want := projectedOctType(goFn.Parameters[index], c.model.Package.Path)
		got := octType(octFn.Parameters[index].Type)
		if want != got {
			return signatureError(c, goFn, octFn, fmt.Sprintf("parameter %d is incompatible: Go maps to %s, Oct expects %s", index+1, want, got))
		}
	}
	wantReturn := "Void"
	if len(goFn.Results) == 1 {
		wantReturn = projectedOctType(goFn.Results[0], c.model.Package.Path)
	}
	gotReturn := octType(octFn.ReturnType)
	if wantReturn != gotReturn {
		return signatureError(c, goFn, octFn, fmt.Sprintf("return is incompatible: Go maps to %s, Oct expects %s", wantReturn, gotReturn))
	}
	return nil
}

func derivedWrapperMetadata(model PackageModel, imports []ast.FunctionDecl) project.WrapperMetadata {
	ordered := append([]ast.FunctionDecl(nil), imports...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Name < ordered[j].Name })
	wrapper := project.WrapperMetadata{
		Name:           "octgo",
		Family:         "OctGo:" + model.Package.Path,
		Protocol:       "octxiliary.v0",
		SidecarCommand: sidecarCommand(model.Package.Name),
		GoModuleDir:    "octgo_bridge",
	}
	for _, imported := range ordered {
		args := make([]string, 0, len(imported.Parameters))
		for _, parameter := range imported.Parameters {
			args = append(args, octType(parameter.Type))
		}
		wrapper.Functions = append(wrapper.Functions, manifestwrapper.FunctionMetadata{
			OctName: imported.Name, WireName: imported.Name, Args: args,
			Return: octType(imported.ReturnType), Fallible: false,
		})
	}
	return wrapper
}

func signatureError(c contract, goFn FunctionModel, octFn ast.FunctionDecl, detail string) error {
	params := make([]string, 0, len(octFn.Parameters))
	for _, parameter := range octFn.Parameters {
		params = append(params, octType(parameter.Type))
	}
	return fmt.Errorf("Go function %s.%s\nhas signature:\n    %s\nOct companion %s expects:\n    fn(%s) -> %s\nerror:\n    %s", c.model.Package.Path, goFn.Name, goFn.Signature, c.path, strings.Join(params, ", "), octType(octFn.ReturnType), detail)
}

func validateConstantsWithConcepts(c contract, selected map[string]ast.TypeRef, report *Report) error {
	bindings := make([]string, 0)
	for _, constant := range c.model.Constants {
		if constant.Type.Package != c.model.Package.Path {
			continue
		}
		if _, ok := selected[constant.Type.Name]; !ok {
			continue
		}
		if !constant.Supported {
			return fmt.Errorf("Go constant %s.%s cannot be checked as Oct concept %s: %s", c.model.Package.Path, constant.Name, constant.Type.Name, constant.UnsupportedReason)
		}
		bindings = append(bindings, fmt.Sprintf("    let OctGo_%s: %s = %s", constant.Name, constant.Type.Name, constant.OctLiteral))
		report.ConstantWitnesses++
	}
	if len(bindings) == 0 {
		return nil
	}
	sourceText := "package " + c.companion.Package + "\nfn OctGoImportedConstantWitnesses() -> Void {\n" + strings.Join(bindings, "\n") + "\n}\n"
	tmp, err := os.CreateTemp("", "zz_octgo_witness_*.octest")
	if err != nil {
		return fmt.Errorf("create bounded OCTGO witness: %w", err)
	}
	path := tmp.Name()
	defer os.Remove(path)
	if _, err := tmp.WriteString(sourceText); err != nil {
		tmp.Close()
		return fmt.Errorf("write bounded OCTGO witness: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close bounded OCTGO witness: %w", err)
	}
	program, err := project.LoadForTestWithSelectedFilesInPackage(path, c.directory, []string{path})
	if err != nil {
		return fmt.Errorf("load bounded OCTGO constant witness: %w", err)
	}
	if err := typecheck.CheckProgram(program); err != nil {
		return fmt.Errorf("Go constant does not satisfy imported Oct Concept: %w", err)
	}
	return nil
}

func parseCompanion(path string) (ast.File, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return ast.File{}, fmt.Errorf("read Oct companion %s: %w", path, err)
	}
	file := source.File{Path: path, Text: string(contents)}
	tokens, err := lex.Analyze(file)
	if err != nil {
		return ast.File{}, err
	}
	return parse.BuildFile(tokens)
}

func contractDirectory(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("inspect OCTGO target %s: %w", abs, err)
	}
	if !info.IsDir() {
		abs = filepath.Dir(abs)
	}
	return abs, nil
}

func octType(ref ast.TypeRef) string {
	if ref.IsArray || ref.ArrayDepth > 0 || ref.Package != "" || ref.Function != nil || ref.VectorOf != nil || ref.MatrixOf != nil || len(ref.TupleOf) != 0 {
		return ref.Name + "[]"
	}
	return ref.Name
}

func projectedOctType(ref TypeRef, packagePath string) string {
	if ref.Name != "" && ref.Package == packagePath {
		return ref.Name
	}
	return ref.Kind
}

func sidecarCommand(packageName string) string {
	var builder strings.Builder
	for _, r := range strings.ToLower(packageName) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
		} else {
			builder.WriteByte('-')
		}
	}
	return "octgo-" + strings.Trim(builder.String(), "-")
}

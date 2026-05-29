package manifestwrapper

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

const SupportedProtocol = "octxiliary.v0"

var supportedTransportTypes = map[string]bool{
	"Void":     true,
	"Int":      true,
	"Float":    true,
	"Bool":     true,
	"String":   true,
	"String[]": true,
	"Bytes":    true,
}

type Metadata struct {
	Name           string
	Family         string
	Protocol       string
	SidecarCommand string
	GoModuleDir    string
	Functions      []FunctionMetadata
}

type FunctionMetadata struct {
	OctName  string
	WireName string
	Args     []string
	Return   string
	Fallible bool
}

func PackageManifestWrappersType() ast.TypeRef {
	return ast.TypeRef{Name: "Wrapper", IsArray: true}
}

func WrapperRequiredFields() map[string]ast.TypeRef {
	return map[string]ast.TypeRef{
		"Name":           {Name: "String"},
		"Family":         {Name: "String"},
		"Protocol":       {Name: "String"},
		"SidecarCommand": {Name: "String"},
		"GoModuleDir":    {Name: "String"},
		"Functions":      {Name: "WrapperFunction", IsArray: true},
	}
}

func WrapperFunctionRequiredFields() map[string]ast.TypeRef {
	return map[string]ast.TypeRef{
		"OctName":  {Name: "String"},
		"WireName": {Name: "String"},
		"Args":     {Name: "String", IsArray: true},
		"Return":   {Name: "String"},
		"Fallible": {Name: "Bool"},
	}
}

func IsSupportedTransportType(t string) bool {
	return supportedTransportTypes[t]
}

func ValidateGoModuleDir(moduleDir string) error {
	if moduleDir == "" {
		return fmt.Errorf("GoModuleDir must be non-empty")
	}
	if filepath.IsAbs(moduleDir) || strings.HasPrefix(moduleDir, "/") || strings.HasPrefix(moduleDir, "\\") || (len(moduleDir) >= 2 && moduleDir[1] == ':') {
		return fmt.Errorf("GoModuleDir must be a relative path")
	}
	for _, part := range strings.Split(strings.ReplaceAll(moduleDir, "\\", "/"), "/") {
		if part == ".." {
			return fmt.Errorf("GoModuleDir must not contain '..' path traversal")
		}
	}
	clean := filepath.Clean(moduleDir)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("GoModuleDir must be a package-local relative path")
	}
	return nil
}

func ValidateWrapperKindRules(kind string, wrappersPresent bool, wrapperCount int) error {
	switch kind {
	case "wrapper":
		if !wrappersPresent {
			return fmt.Errorf("Kind \"wrapper\" requires Wrappers metadata")
		}
		if wrapperCount == 0 {
			return fmt.Errorf("Kind \"wrapper\" requires non-empty Wrappers metadata")
		}
	case "pure", "experiment":
		if wrapperCount > 0 {
			return fmt.Errorf("Kind %q does not allow non-empty Wrappers metadata", kind)
		}
	}
	return nil
}

func ExtractWrappers(expr ast.Expr, wrapperFields map[string]bool, functionFields map[string]bool) ([]Metadata, error) {
	wrappersExpr, ok := expr.(ast.ArrayLiteralExpr)
	if !ok {
		return nil, fmt.Errorf("manifest field 'Wrappers' must be a Wrapper[] literal")
	}
	wrappers := make([]Metadata, 0, len(wrappersExpr.Elements))
	seenNames := map[string]bool{}
	seenFamilies := map[string]bool{}
	for idx, item := range wrappersExpr.Elements {
		wrapperExpr, ok := item.(ast.RecordLiteralExpr)
		if !ok || wrapperExpr.TypeName != "Wrapper" {
			return nil, fmt.Errorf("manifest wrapper at index %d must be a Wrapper literal", idx)
		}
		wrapper, err := extractWrapper(wrapperExpr, wrapperFields, functionFields)
		if err != nil {
			return nil, fmt.Errorf("manifest wrapper at index %d: %w", idx, err)
		}
		if seenNames[wrapper.Name] {
			return nil, fmt.Errorf("manifest wrapper at index %d: duplicate Wrapper.Name %q", idx, wrapper.Name)
		}
		seenNames[wrapper.Name] = true
		if seenFamilies[wrapper.Family] {
			return nil, fmt.Errorf("manifest wrapper at index %d: duplicate Wrapper.Family %q", idx, wrapper.Family)
		}
		seenFamilies[wrapper.Family] = true
		wrappers = append(wrappers, wrapper)
	}
	return wrappers, nil
}

func extractWrapper(record ast.RecordLiteralExpr, wrapperFields map[string]bool, functionFields map[string]bool) (Metadata, error) {
	fields, err := literalFields(record, wrapperFields)
	if err != nil {
		return Metadata{}, err
	}
	name, err := requiredNonEmptyString(fields, "Name")
	if err != nil {
		return Metadata{}, err
	}
	family, err := requiredNonEmptyString(fields, "Family")
	if err != nil {
		return Metadata{}, err
	}
	protocol, err := requiredNonEmptyString(fields, "Protocol")
	if err != nil {
		return Metadata{}, err
	}
	if protocol != SupportedProtocol {
		return Metadata{}, fmt.Errorf("Protocol must be %q", SupportedProtocol)
	}
	sidecarCommand, err := requiredNonEmptyString(fields, "SidecarCommand")
	if err != nil {
		return Metadata{}, err
	}
	goModuleDir, err := requiredNonEmptyString(fields, "GoModuleDir")
	if err != nil {
		return Metadata{}, err
	}
	if err := ValidateGoModuleDir(goModuleDir); err != nil {
		return Metadata{}, err
	}
	functionsExpr, ok := fields["Functions"].(ast.ArrayLiteralExpr)
	if !ok {
		return Metadata{}, fmt.Errorf("Functions must be a WrapperFunction[] literal")
	}
	if len(functionsExpr.Elements) == 0 {
		return Metadata{}, fmt.Errorf("Functions must be non-empty")
	}
	functions, err := extractFunctions(functionsExpr, functionFields)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Name: name, Family: family, Protocol: protocol, SidecarCommand: sidecarCommand, GoModuleDir: goModuleDir, Functions: functions}, nil
}

func extractFunctions(functionsExpr ast.ArrayLiteralExpr, functionFields map[string]bool) ([]FunctionMetadata, error) {
	functions := make([]FunctionMetadata, 0, len(functionsExpr.Elements))
	seenOctNames := map[string]bool{}
	seenWireNames := map[string]bool{}
	for idx, item := range functionsExpr.Elements {
		functionExpr, ok := item.(ast.RecordLiteralExpr)
		if !ok || functionExpr.TypeName != "WrapperFunction" {
			return nil, fmt.Errorf("function at index %d must be a WrapperFunction literal", idx)
		}
		function, err := extractFunction(functionExpr, functionFields)
		if err != nil {
			return nil, fmt.Errorf("function at index %d: %w", idx, err)
		}
		if seenOctNames[function.OctName] {
			return nil, fmt.Errorf("function at index %d: duplicate OctName %q", idx, function.OctName)
		}
		seenOctNames[function.OctName] = true
		if seenWireNames[function.WireName] {
			return nil, fmt.Errorf("function at index %d: duplicate WireName %q", idx, function.WireName)
		}
		seenWireNames[function.WireName] = true
		functions = append(functions, function)
	}
	return functions, nil
}

func extractFunction(record ast.RecordLiteralExpr, functionFields map[string]bool) (FunctionMetadata, error) {
	fields, err := literalFields(record, functionFields)
	if err != nil {
		return FunctionMetadata{}, err
	}
	octName, err := requiredNonEmptyString(fields, "OctName")
	if err != nil {
		return FunctionMetadata{}, err
	}
	wireName, err := requiredNonEmptyString(fields, "WireName")
	if err != nil {
		return FunctionMetadata{}, err
	}
	argsExpr, ok := fields["Args"].(ast.ArrayLiteralExpr)
	if !ok {
		return FunctionMetadata{}, fmt.Errorf("Args must be a String[] literal")
	}
	args := make([]string, 0, len(argsExpr.Elements))
	for idx, arg := range argsExpr.Elements {
		argLiteral, ok := arg.(ast.StringLiteralExpr)
		if !ok {
			return FunctionMetadata{}, fmt.Errorf("Args element at index %d must be a string literal", idx)
		}
		if !IsSupportedTransportType(argLiteral.Value) {
			return FunctionMetadata{}, fmt.Errorf("Args element at index %d has unsupported transport type %q", idx, argLiteral.Value)
		}
		args = append(args, argLiteral.Value)
	}
	returnType, err := requiredNonEmptyString(fields, "Return")
	if err != nil {
		return FunctionMetadata{}, err
	}
	if !IsSupportedTransportType(returnType) {
		return FunctionMetadata{}, fmt.Errorf("Return has unsupported transport type %q", returnType)
	}
	fallible, ok := fields["Fallible"].(ast.BoolLiteral)
	if !ok {
		return FunctionMetadata{}, fmt.Errorf("Fallible must be a Bool literal")
	}
	return FunctionMetadata{OctName: octName, WireName: wireName, Args: args, Return: returnType, Fallible: fallible.Value}, nil
}

func literalFields(record ast.RecordLiteralExpr, allowed map[string]bool) (map[string]ast.Expr, error) {
	fields := make(map[string]ast.Expr, len(record.Fields))
	for _, field := range record.Fields {
		if !allowed[field.Name] {
			return nil, fmt.Errorf("literal has unsupported field '%s'", field.Name)
		}
		if _, exists := fields[field.Name]; exists {
			return nil, fmt.Errorf("literal contains duplicate field '%s'", field.Name)
		}
		fields[field.Name] = field.Value
	}
	for field := range allowed {
		if _, ok := fields[field]; !ok {
			return nil, fmt.Errorf("literal missing required field '%s'", field)
		}
	}
	return fields, nil
}

func requiredNonEmptyString(fields map[string]ast.Expr, fieldName string) (string, error) {
	expr, ok := fields[fieldName]
	if !ok {
		return "", fmt.Errorf("%s is required", fieldName)
	}
	literal, ok := expr.(ast.StringLiteralExpr)
	if !ok {
		return "", fmt.Errorf("%s must be a string literal", fieldName)
	}
	if literal.Value == "" {
		return "", fmt.Errorf("%s must be non-empty", fieldName)
	}
	return literal.Value, nil
}

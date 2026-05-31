package manifestwrapper

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
)

const SupportedProtocol = "octxiliary.v0"

var supportedTransportTypes = map[string]bool{
	"Void":       true,
	"Int":        true,
	"Float":      true,
	"Bool":       true,
	"String":     true,
	"String[]":   true,
	"String[][]": true,
	"Float[]":    true,
	"Bytes":      true,
}

type Metadata struct {
	Name           string
	Family         string
	Protocol       string
	SidecarCommand string
	GoModuleDir    string
	TransportTypes []TransportTypeMetadata
	Functions      []FunctionMetadata
}

type TransportTypeMetadata struct {
	Name   string
	Kind   string
	Fields []TransportFieldMetadata
}

type TransportFieldMetadata struct {
	Name string
	Type string
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

func WrapperOptionalFields() map[string]ast.TypeRef {
	return map[string]ast.TypeRef{
		"TransportTypes": {Name: "WrapperTransportType", IsArray: true},
	}
}

func WrapperTransportTypeRequiredFields() map[string]ast.TypeRef {
	return map[string]ast.TypeRef{
		"Name":   {Name: "String"},
		"Kind":   {Name: "String"},
		"Fields": {Name: "WrapperTransportField", IsArray: true},
	}
}

func WrapperTransportFieldRequiredFields() map[string]ast.TypeRef {
	return map[string]ast.TypeRef{
		"Name": {Name: "String"},
		"Type": {Name: "String"},
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
	if strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">") {
		return true
	}
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
	fields, err := wrapperLiteralFields(record, wrapperFields)
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
	transportTypes, err := extractTransportTypes(fields["TransportTypes"])
	if err != nil {
		return Metadata{}, err
	}
	functionsExpr, ok := fields["Functions"].(ast.ArrayLiteralExpr)
	if !ok {
		return Metadata{}, fmt.Errorf("Functions must be a WrapperFunction[] literal")
	}
	if len(functionsExpr.Elements) == 0 {
		return Metadata{}, fmt.Errorf("Functions must be non-empty")
	}
	functions, err := extractFunctions(functionsExpr, functionFields, transportTypes)
	if err != nil {
		return Metadata{}, err
	}
	return Metadata{Name: name, Family: family, Protocol: protocol, SidecarCommand: sidecarCommand, GoModuleDir: goModuleDir, TransportTypes: transportTypes, Functions: functions}, nil
}

func extractFunctions(functionsExpr ast.ArrayLiteralExpr, functionFields map[string]bool, transportTypes []TransportTypeMetadata) ([]FunctionMetadata, error) {
	functions := make([]FunctionMetadata, 0, len(functionsExpr.Elements))
	seenOctNames := map[string]bool{}
	seenWireNames := map[string]bool{}
	for idx, item := range functionsExpr.Elements {
		functionExpr, ok := item.(ast.RecordLiteralExpr)
		if !ok || functionExpr.TypeName != "WrapperFunction" {
			return nil, fmt.Errorf("function at index %d must be a WrapperFunction literal", idx)
		}
		function, err := extractFunction(functionExpr, functionFields, transportTypes)
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

func extractFunction(record ast.RecordLiteralExpr, functionFields map[string]bool, transportTypes []TransportTypeMetadata) (FunctionMetadata, error) {
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
		if !IsSupportedTransportType(argLiteral.Value) && !isDeclaredTransportType(argLiteral.Value, transportTypes) {
			return FunctionMetadata{}, fmt.Errorf("Args element at index %d has unsupported transport type %q", idx, argLiteral.Value)
		}
		args = append(args, argLiteral.Value)
	}
	returnType, err := requiredNonEmptyString(fields, "Return")
	if err != nil {
		return FunctionMetadata{}, err
	}
	if declared, ok := findDeclaredTransportType(returnType, transportTypes); ok {
		if declared.Kind != "handle" {
			return FunctionMetadata{}, fmt.Errorf("Return uses declared record transport type %q; record returns are not supported", returnType)
		}
	} else if !IsSupportedTransportType(returnType) {
		return FunctionMetadata{}, fmt.Errorf("Return has unsupported transport type %q", returnType)
	}
	fallible, ok := fields["Fallible"].(ast.BoolLiteral)
	if !ok {
		return FunctionMetadata{}, fmt.Errorf("Fallible must be a Bool literal")
	}
	return FunctionMetadata{OctName: octName, WireName: wireName, Args: args, Return: returnType, Fallible: fallible.Value}, nil
}

func extractTransportTypes(expr ast.Expr) ([]TransportTypeMetadata, error) {
	if expr == nil {
		return nil, nil
	}
	array, ok := expr.(ast.ArrayLiteralExpr)
	if !ok {
		return nil, fmt.Errorf("TransportTypes must be a WrapperTransportType[] literal")
	}
	out := make([]TransportTypeMetadata, 0, len(array.Elements))
	seen := map[string]bool{}
	for idx, item := range array.Elements {
		record, ok := item.(ast.RecordLiteralExpr)
		if !ok || record.TypeName != "WrapperTransportType" {
			return nil, fmt.Errorf("TransportTypes element at index %d must be a WrapperTransportType literal", idx)
		}
		fields, err := literalFields(record, map[string]bool{"Name": true, "Kind": true, "Fields": true})
		if err != nil {
			return nil, fmt.Errorf("TransportTypes element at index %d: %w", idx, err)
		}
		name, err := requiredNonEmptyString(fields, "Name")
		if err != nil {
			return nil, fmt.Errorf("TransportTypes element at index %d: %w", idx, err)
		}
		if seen[name] {
			return nil, fmt.Errorf("TransportTypes element at index %d: duplicate transport type name %q", idx, name)
		}
		seen[name] = true
		kind, err := requiredNonEmptyString(fields, "Kind")
		if err != nil {
			return nil, fmt.Errorf("TransportTypes element at index %d: %w", idx, err)
		}
		if kind != "record" && kind != "handle" {
			return nil, fmt.Errorf("TransportTypes element at index %d: Kind must be %q or %q", idx, "record", "handle")
		}
		fieldArray, ok := fields["Fields"].(ast.ArrayLiteralExpr)
		if !ok {
			return nil, fmt.Errorf("TransportTypes element at index %d: Fields must be a WrapperTransportField[] literal", idx)
		}
		transportFields, err := extractTransportFields(fieldArray)
		if err != nil {
			return nil, fmt.Errorf("TransportTypes element at index %d: %w", idx, err)
		}
		if kind == "handle" {
			if len(transportFields) != 1 {
				return nil, fmt.Errorf("TransportTypes element at index %d: handle transport type must have exactly one field", idx)
			}
			if transportFields[0].Name != "Handle" {
				return nil, fmt.Errorf("TransportTypes element at index %d: handle transport field must be named Handle", idx)
			}
			if transportFields[0].Type != "Int" {
				return nil, fmt.Errorf("TransportTypes element at index %d: handle transport field Handle must have type Int", idx)
			}
		}
		out = append(out, TransportTypeMetadata{Name: name, Kind: kind, Fields: transportFields})
	}
	return out, nil
}

func extractTransportFields(array ast.ArrayLiteralExpr) ([]TransportFieldMetadata, error) {
	if len(array.Elements) == 0 {
		return nil, fmt.Errorf("Fields must be non-empty")
	}
	out := make([]TransportFieldMetadata, 0, len(array.Elements))
	seen := map[string]bool{}
	for idx, item := range array.Elements {
		record, ok := item.(ast.RecordLiteralExpr)
		if !ok || record.TypeName != "WrapperTransportField" {
			return nil, fmt.Errorf("Fields element at index %d must be a WrapperTransportField literal", idx)
		}
		fields, err := literalFields(record, map[string]bool{"Name": true, "Type": true})
		if err != nil {
			return nil, fmt.Errorf("Fields element at index %d: %w", idx, err)
		}
		name, err := requiredNonEmptyString(fields, "Name")
		if err != nil {
			return nil, fmt.Errorf("Fields element at index %d: %w", idx, err)
		}
		if seen[name] {
			return nil, fmt.Errorf("Fields element at index %d: duplicate field name %q", idx, name)
		}
		seen[name] = true
		typ, err := requiredNonEmptyString(fields, "Type")
		if err != nil {
			return nil, fmt.Errorf("Fields element at index %d: %w", idx, err)
		}
		if !IsSupportedTransportFieldType(typ) {
			return nil, fmt.Errorf("Fields element at index %d has unsupported transport field type %q", idx, typ)
		}
		out = append(out, TransportFieldMetadata{Name: name, Type: typ})
	}
	return out, nil
}

func IsSupportedTransportFieldType(t string) bool {
	if t == "Void" {
		return false
	}
	if IsSupportedTransportType(t) {
		return true
	}
	return strings.HasPrefix(t, "Int<") && strings.HasSuffix(t, ">")
}

func isDeclaredTransportType(name string, types []TransportTypeMetadata) bool {
	_, ok := findDeclaredTransportType(name, types)
	return ok
}

func findDeclaredTransportType(name string, types []TransportTypeMetadata) (TransportTypeMetadata, bool) {
	for _, typ := range types {
		if typ.Name == name {
			return typ, true
		}
	}
	return TransportTypeMetadata{}, false
}

func wrapperLiteralFields(record ast.RecordLiteralExpr, allowed map[string]bool) (map[string]ast.Expr, error) {
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
	for _, field := range []string{"Name", "Family", "Protocol", "SidecarCommand", "GoModuleDir", "Functions"} {
		if _, ok := fields[field]; !ok {
			return nil, fmt.Errorf("literal missing required field '%s'", field)
		}
	}
	return fields, nil
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

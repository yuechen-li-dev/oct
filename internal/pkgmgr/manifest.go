package pkgmgr

import (
	"fmt"
	"sort"

	"oct/internal/ast"
	"oct/internal/lex"
	"oct/internal/parse"
	"oct/internal/source"
)

type ManifestMetadata struct {
	Name           string
	Version        string
	Description    string
	Kind           string
	EntryMilestone string
	Dependencies   []DependencyMetadata
}

type DependencyMetadata struct {
	Name               string `json:"name"`
	VersionRequirement string `json:"version_requirement"`
	Source             string `json:"source,omitempty"`
}

func loadManifestMetadata(path string) (ManifestMetadata, error) {
	file, err := source.Load(path)
	if err != nil {
		return ManifestMetadata{}, fmt.Errorf("read manifest %s: %w", path, err)
	}
	lexed, err := lex.Analyze(file)
	if err != nil {
		return ManifestMetadata{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	parsed, err := parse.BuildFile(lexed)
	if err != nil {
		return ManifestMetadata{}, fmt.Errorf("parse manifest %s: %w", path, err)
	}
	return extractManifestMetadata(parsed)
}

func extractManifestMetadata(file ast.File) (ManifestMetadata, error) {
	if file.Package != "Manifest" {
		return ManifestMetadata{}, fmt.Errorf("manifest.oct must declare package Manifest")
	}
	if err := requireRecordShape(file, "PackageManifest", map[string]ast.TypeRef{
		"Name":         {Name: "String"},
		"Version":      {Name: "String"},
		"Description":  {Name: "String"},
		"Dependencies": {Name: "Dependency", IsArray: true},
	}, map[string]ast.TypeRef{
		"Kind":           {Name: "String"},
		"EntryMilestone": {Name: "String"},
	}); err != nil {
		return ManifestMetadata{}, err
	}
	if err := requireRecordShape(file, "Dependency", map[string]ast.TypeRef{
		"Name":               {Name: "String"},
		"VersionRequirement": {Name: "String"},
	}, map[string]ast.TypeRef{
		"Source": {Name: "String"},
	}); err != nil {
		return ManifestMetadata{}, err
	}
	manifestFn, ok := findManifestFunction(file)
	if !ok {
		return ManifestMetadata{}, fmt.Errorf("manifest.oct must define fn Manifest() -> PackageManifest")
	}
	return extractManifestReturn(manifestFn)
}

func requireRecordShape(file ast.File, recordName string, required map[string]ast.TypeRef, optional map[string]ast.TypeRef) error {
	record, ok := findRecord(file, recordName)
	if !ok {
		return fmt.Errorf("manifest.oct must define record %s", recordName)
	}
	minFields := len(required)
	maxFields := len(required) + len(optional)
	if len(record.Fields) < minFields || len(record.Fields) > maxFields {
		if len(optional) == 0 {
			return fmt.Errorf("manifest.oct record %s must define exactly %d fields", recordName, minFields)
		}
		return fmt.Errorf("manifest.oct record %s must define %d-%d fields", recordName, minFields, maxFields)
	}
	seen := map[string]bool{}
	for _, field := range record.Fields {
		requiredType, ok := required[field.Name]
		if !ok {
			requiredType, ok = optional[field.Name]
		}
		if !ok {
			return fmt.Errorf("manifest.oct record %s contains unsupported field '%s'", recordName, field.Name)
		}
		if seen[field.Name] {
			return fmt.Errorf("manifest.oct record %s contains duplicate field '%s'", recordName, field.Name)
		}
		seen[field.Name] = true
		if field.Type.Name != requiredType.Name || field.Type.Package != "" || field.Type.IsArray != requiredType.IsArray {
			return fmt.Errorf("manifest.oct record %s field '%s' must be %s", recordName, field.Name, typeLabel(requiredType))
		}
	}
	for field := range required {
		if !seen[field] {
			return fmt.Errorf("manifest.oct record %s missing required field '%s'", recordName, field)
		}
	}
	return nil
}

func typeLabel(ref ast.TypeRef) string {
	if ref.IsArray {
		return ref.Name + "[]"
	}
	return ref.Name
}

func findRecord(file ast.File, recordName string) (ast.RecordDecl, bool) {
	for _, record := range file.Records {
		if record.Name == recordName {
			return record, true
		}
	}
	return ast.RecordDecl{}, false
}

func findManifestFunction(file ast.File) (ast.FunctionDecl, bool) {
	for _, fn := range file.Functions {
		if fn.Name != "Manifest" {
			continue
		}
		if len(fn.Parameters) != 0 || fn.ReturnType.Package != "" || fn.ReturnType.IsArray || fn.ReturnType.Name != "PackageManifest" {
			continue
		}
		return fn, true
	}
	return ast.FunctionDecl{}, false
}

func extractManifestReturn(fn ast.FunctionDecl) (ManifestMetadata, error) {
	if len(fn.Body.Statements) != 1 {
		return ManifestMetadata{}, fmt.Errorf("manifest function must contain a single return statement")
	}
	ret, ok := fn.Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		return ManifestMetadata{}, fmt.Errorf("manifest function must contain a single return statement")
	}
	recordExpr, ok := ret.Value.(ast.RecordLiteralExpr)
	if !ok || recordExpr.TypeName != "PackageManifest" {
		return ManifestMetadata{}, fmt.Errorf("manifest function must return PackageManifest literal")
	}

	fieldValues := map[string]ast.Expr{}
	for _, field := range recordExpr.Fields {
		fieldValues[field.Name] = field.Value
	}
	name, err := stringField(fieldValues, "Name")
	if err != nil {
		return ManifestMetadata{}, err
	}
	version, err := stringField(fieldValues, "Version")
	if err != nil {
		return ManifestMetadata{}, err
	}
	description, err := stringField(fieldValues, "Description")
	if err != nil {
		return ManifestMetadata{}, err
	}
	kind, err := optionalStringField(fieldValues, "Kind")
	if err != nil {
		return ManifestMetadata{}, err
	}
	entryMilestone, err := optionalStringField(fieldValues, "EntryMilestone")
	if err != nil {
		return ManifestMetadata{}, err
	}
	depsExpr, ok := fieldValues["Dependencies"].(ast.ArrayLiteralExpr)
	if !ok {
		return ManifestMetadata{}, fmt.Errorf("manifest field 'Dependencies' must be a Dependency[] literal")
	}
	deps := make([]DependencyMetadata, 0, len(depsExpr.Elements))
	for idx, expr := range depsExpr.Elements {
		recordDep, ok := expr.(ast.RecordLiteralExpr)
		if !ok || recordDep.TypeName != "Dependency" {
			return ManifestMetadata{}, fmt.Errorf("manifest dependency at index %d must be a Dependency literal", idx)
		}
		depFields := map[string]ast.Expr{}
		for _, field := range recordDep.Fields {
			depFields[field.Name] = field.Value
		}
		depName, err := stringField(depFields, "Name")
		if err != nil {
			return ManifestMetadata{}, fmt.Errorf("manifest dependency at index %d: %w", idx, err)
		}
		depReq, err := stringField(depFields, "VersionRequirement")
		if err != nil {
			return ManifestMetadata{}, fmt.Errorf("manifest dependency at index %d: %w", idx, err)
		}
		depSource, err := optionalStringField(depFields, "Source")
		if err != nil {
			return ManifestMetadata{}, fmt.Errorf("manifest dependency at index %d: %w", idx, err)
		}
		if len(depFields) != 2 && len(depFields) != 3 {
			keys := make([]string, 0, len(depFields))
			for key := range depFields {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			return ManifestMetadata{}, fmt.Errorf("manifest dependency at index %d has unsupported fields", idx)
		}
		deps = append(deps, DependencyMetadata{Name: depName, VersionRequirement: depReq, Source: depSource})
	}
	if len(fieldValues) < 4 || len(fieldValues) > 6 {
		return ManifestMetadata{}, fmt.Errorf("manifest return literal has unsupported fields")
	}
	return ManifestMetadata{
		Name:           name,
		Version:        version,
		Description:    description,
		Kind:           kind,
		EntryMilestone: entryMilestone,
		Dependencies:   deps,
	}, nil
}

func stringField(fields map[string]ast.Expr, fieldName string) (string, error) {
	expr, ok := fields[fieldName]
	if !ok {
		return "", fmt.Errorf("manifest field '%s' is required", fieldName)
	}
	str, ok := expr.(ast.StringLiteralExpr)
	if !ok {
		return "", fmt.Errorf("manifest field '%s' must be a string literal", fieldName)
	}
	return str.Value, nil
}

func optionalStringField(fields map[string]ast.Expr, fieldName string) (string, error) {
	expr, ok := fields[fieldName]
	if !ok {
		return "", nil
	}
	str, ok := expr.(ast.StringLiteralExpr)
	if !ok {
		return "", fmt.Errorf("manifest field '%s' must be a string literal", fieldName)
	}
	return str.Value, nil
}

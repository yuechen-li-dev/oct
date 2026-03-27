package project

import (
	"fmt"
	"sort"

	"oct/internal/ast"
)

func validateManifestFile(packageName string, file ast.File) error {
	if file.Package != "Manifest" {
		return fmt.Errorf("manifest.oct must declare package Manifest")
	}
	if err := requirePackageManifestRecord(file); err != nil {
		return err
	}
	if err := requireDependencyRecord(file); err != nil {
		return err
	}
	manifestFn, ok := findManifestFunction(file)
	if !ok {
		return fmt.Errorf("manifest.oct must define fn Manifest() -> PackageManifest")
	}
	if err := validateManifestFunctionBody(packageName, manifestFn); err != nil {
		return err
	}
	return nil
}

func requirePackageManifestRecord(file ast.File) error {
	record, ok := findRecord(file, "PackageManifest")
	if !ok {
		return fmt.Errorf("manifest.oct must define record PackageManifest")
	}
	required := map[string]ast.TypeRef{
		"Name":         {Name: "String"},
		"Version":      {Name: "String"},
		"Description":  {Name: "String"},
		"Dependencies": {Name: "Dependency", IsArray: true},
	}
	if err := validateRecordFields(record, required); err != nil {
		return fmt.Errorf("manifest.oct has invalid PackageManifest record: %w", err)
	}
	return nil
}

func requireDependencyRecord(file ast.File) error {
	record, ok := findRecord(file, "Dependency")
	if !ok {
		return fmt.Errorf("manifest.oct must define record Dependency")
	}
	required := map[string]ast.TypeRef{
		"Name":               {Name: "String"},
		"VersionRequirement": {Name: "String"},
	}
	if err := validateRecordFields(record, required); err != nil {
		return fmt.Errorf("manifest.oct has invalid Dependency record: %w", err)
	}
	return nil
}

func validateRecordFields(record ast.RecordDecl, required map[string]ast.TypeRef) error {
	if len(record.Fields) != len(required) {
		return fmt.Errorf("expected %d fields, got %d", len(required), len(record.Fields))
	}
	for _, field := range record.Fields {
		requiredType, ok := required[field.Name]
		if !ok {
			return fmt.Errorf("unexpected field '%s'", field.Name)
		}
		if field.Type.Name != requiredType.Name || field.Type.IsArray != requiredType.IsArray || field.Type.Package != "" {
			return fmt.Errorf("field '%s' has wrong type", field.Name)
		}
	}
	return nil
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
		if len(fn.Parameters) != 0 || fn.ReturnType.Name != "PackageManifest" || fn.ReturnType.Package != "" || fn.ReturnType.IsArray {
			continue
		}
		return fn, true
	}
	return ast.FunctionDecl{}, false
}

func validateManifestFunctionBody(packageName string, fn ast.FunctionDecl) error {
	if len(fn.Body.Statements) != 1 {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	ret, ok := fn.Body.Statements[0].(ast.ReturnStmt)
	if !ok {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	recordExpr, ok := ret.Value.(ast.RecordLiteralExpr)
	if !ok || recordExpr.TypeName != "PackageManifest" {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	fields := make(map[string]ast.Expr)
	for _, field := range recordExpr.Fields {
		fields[field.Name] = field.Value
	}
	if nameExpr, ok := fields["Name"].(ast.StringLiteralExpr); !ok {
		return fmt.Errorf("manifest function returned invalid package metadata")
	} else if nameExpr.Value != packageName {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	if _, ok := fields["Version"].(ast.StringLiteralExpr); !ok {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	if _, ok := fields["Description"].(ast.StringLiteralExpr); !ok {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	deps, ok := fields["Dependencies"].(ast.ArrayLiteralExpr)
	if !ok {
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	for _, dep := range deps.Elements {
		recordDep, ok := dep.(ast.RecordLiteralExpr)
		if !ok || recordDep.TypeName != "Dependency" {
			return fmt.Errorf("manifest function returned invalid package metadata")
		}
		depFields := make(map[string]ast.Expr)
		for _, field := range recordDep.Fields {
			depFields[field.Name] = field.Value
		}
		if _, ok := depFields["Name"].(ast.StringLiteralExpr); !ok {
			return fmt.Errorf("manifest function returned invalid package metadata")
		}
		if _, ok := depFields["VersionRequirement"].(ast.StringLiteralExpr); !ok {
			return fmt.Errorf("manifest function returned invalid package metadata")
		}
		if len(depFields) != 2 {
			return fmt.Errorf("manifest function returned invalid package metadata")
		}
	}
	if len(fields) != 4 {
		keys := make([]string, 0, len(fields))
		for key := range fields {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return fmt.Errorf("manifest function returned invalid package metadata")
	}
	return nil
}

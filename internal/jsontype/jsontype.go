// Package jsontype describes an Oct type as the Json builtins read and write
// it: it turns a written type into an octjson.Schema.
//
// It is the one place that does so. The typechecker calls it to refuse a
// type that has no JSON form, and the interpreter and the compiled lane call
// it for the schema they hand to octjson, so the three cannot describe one
// type differently.
package jsontype

import (
	"strings"

	"github.com/yuechen-li-dev/oct/internal/ast"
	"github.com/yuechen-li-dev/oct/internal/octjson"
	"github.com/yuechen-li-dev/oct/internal/project"
)

// Of describes the type written as t in package pkg of program. It always
// answers: a type, or a part of one, that has no JSON form is a Schema of
// octjson.KindUnsupported, which octjson.Check reports by name.
func Of(program project.Program, pkg string, t ast.TypeRef) *octjson.Schema {
	b := builder{program: program, named: map[string]*octjson.Schema{}}
	return b.of(pkg, t)
}

// tableRowPrefix begins the name the typechecker gives the type of one row
// of a record table: `__oct_table_row_Ticket` for a row of `Ticket`. No
// source can write the name; it is the type of `tickets[0]`.
const tableRowPrefix = "__oct_table_row_"

type builder struct {
	program project.Program
	// named holds the schema of every record, table and enum met so far,
	// by `Package.Name`, so that a type that names itself ends.
	named map[string]*octjson.Schema
}

func unsupported(what string) *octjson.Schema {
	return &octjson.Schema{Kind: octjson.KindUnsupported, Name: what}
}

func (b *builder) of(pkg string, t ast.TypeRef) *octjson.Schema {
	if t.IsArray || t.ArrayDepth > 0 {
		element := t
		element.ArrayDepth = max(t.ArrayDepth, 1) - 1
		element.IsArray = element.ArrayDepth > 0
		return &octjson.Schema{Kind: octjson.KindArray, Elem: b.of(pkg, element)}
	}
	switch {
	case t.Function != nil:
		return unsupported("a function value")
	case len(t.TupleOf) > 0:
		return unsupported("a tuple")
	case t.FlowInstanceOf != nil:
		return unsupported("a flow instance")
	case t.VectorOf != nil:
		return &octjson.Schema{Kind: octjson.KindVector, Elem: b.of(pkg, *t.VectorOf)}
	case t.MatrixOf != nil:
		return &octjson.Schema{Kind: octjson.KindMatrix, Elem: b.of(pkg, *t.MatrixOf)}
	case ast.IsOptionType(t):
		return &octjson.Schema{Kind: octjson.KindOption, Elem: b.of(pkg, t.TypeArguments[0])}
	}
	if t.Package == "" {
		switch t.Name {
		case "Bool":
			return &octjson.Schema{Kind: octjson.KindBool}
		case "Int":
			return &octjson.Schema{Kind: octjson.KindInt, Dimension: t.Dimension.String()}
		case "Float":
			return &octjson.Schema{Kind: octjson.KindFloat, Dimension: t.Dimension.String()}
		case "String":
			return &octjson.Schema{Kind: octjson.KindString}
		}
	}

	owner := pkg
	if t.Package != "" {
		owner = t.Package
	}
	key := owner + "." + t.Name
	if schema, met := b.named[key]; met {
		return schema
	}
	declared := b.program.Packages[owner]
	// One row of a record table, `tickets[0]`, is a record of the table's
	// cells, and is named after the table.
	name, isRow := strings.CutPrefix(t.Name, tableRowPrefix)
	for _, record := range declared.Records {
		if record.Name != name || (isRow && !record.IsTable) {
			continue
		}
		schema := &octjson.Schema{Kind: octjson.KindRecord, Name: name}
		if record.IsTable && !isRow {
			schema.Kind = octjson.KindTable
		}
		b.named[key] = schema
		for _, field := range record.Fields {
			schema.Fields = append(schema.Fields, octjson.Field{Name: field.Name, Type: b.of(owner, field.Type)})
		}
		return schema
	}
	for _, enum := range declared.Enums {
		if enum.Name != t.Name {
			continue
		}
		schema := &octjson.Schema{Kind: octjson.KindEnum, Name: t.Name}
		for _, variant := range enum.Variants {
			if variant.Payload != nil {
				return unsupported("the enum " + t.Name + ", whose variant " + variant.Name + " carries a payload,")
			}
			schema.Variants = append(schema.Variants, variant.Name)
		}
		b.named[key] = schema
		return schema
	}
	for _, concept := range declared.Concepts {
		if concept.Name != t.Name {
			continue
		}
		base := b.of(owner, concept.Target)
		if len(concept.Requirements) == 0 {
			// A transparent alias, where concept expansion left one named.
			return base
		}
		// A refined concept is its base type in the document, and a value
		// of it is admitted as it is read.
		refined := *base
		refined.Concept = key
		return &refined
	}
	return unsupported(t.Name)
}

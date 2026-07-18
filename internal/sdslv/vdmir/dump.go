package vdmir

import (
	"fmt"
	"strings"
)

func Dump(module Module) string {
	var b strings.Builder
	line := func(indent int, text string) {
		for i := 0; i < indent; i++ {
			b.WriteString("  ")
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}

	if module.Namespace != "" {
		line(0, "vdmir module "+module.Namespace)
	} else {
		line(0, "vdmir module <anonymous>")
	}
	for _, requirement := range module.Requirements {
		line(0, fmt.Sprintf("require %s scope=%s tuple=%dx%dx%d components=%s,%s,%s,%s packing=%s layout=%s",
			requirement.Kind, requirement.Scope, requirement.M, requirement.N, requirement.K,
			requirement.AComponent, requirement.BComponent, requirement.CComponent, requirement.Result,
			requirement.InputPacking, requirement.LogicalLayout))
	}
	for _, alias := range module.TypeAliases {
		line(0, fmt.Sprintf("typealias %s = %s", alias.Name, FormatType(alias.Target)))
	}
	for _, record := range module.Records {
		line(0, "record "+record.Name)
		for _, field := range record.Fields {
			line(1, fmt.Sprintf("field %s: %s", field.Name, FormatType(field.Type)))
		}
	}
	for _, board := range module.Boards {
		line(0, "board "+board.Name)
		for _, field := range board.Fields {
			line(1, fmt.Sprintf("field %s: %s", field.Name, FormatType(field.Type)))
		}
	}
	for _, stream := range module.Streams {
		line(0, "stream "+stream.Name)
		for _, field := range stream.Fields {
			line(1, fmt.Sprintf("field %s: %s", field.Name, FormatType(field.Type)))
		}
	}
	for _, enum := range module.Enums {
		line(0, "enum "+enum.Name)
		for _, variant := range enum.Variants {
			if !variant.HasPayload {
				line(1, "variant "+variant.Name)
				continue
			}
			line(1, "variant "+variant.Name)
			for _, field := range variant.Payload {
				line(2, fmt.Sprintf("field %s: %s", field.Name, FormatType(field.Type)))
			}
		}
	}
	for _, resource := range module.Resources {
		text := fmt.Sprintf("resource %s %s: %s", resource.Access, resource.Name, FormatType(Type{Kind: TypeRuntimeArray, Element: resource.ElementTypeRef()}))
		if resource.BundleName != "" {
			text += " bundle " + resource.BundleName
		}
		text += fmt.Sprintf(" binding(%d,%d)", resource.Binding.Binding, resource.Binding.Set)
		if resource.Binding.Explicit {
			text += " explicit"
		}
		line(0, text)
	}
	for _, workgroup := range module.Workgroups {
		line(0, fmt.Sprintf("workgroup %s: %s shader %s", workgroup.Name, FormatType(workgroup.Type), workgroup.ShaderName))
	}
	for _, fn := range module.Functions {
		line(0, fmt.Sprintf("function %s -> %s emitted %s", fn.Name, FormatType(fn.ReturnType), fn.EmittedName))
		for _, param := range fn.Params {
			line(1, fmt.Sprintf("param %s: %s", param.Name, FormatType(param.Type)))
		}
		for _, local := range fn.Locals {
			line(1, fmt.Sprintf("local %s: %s", local.Name, FormatType(local.Type)))
		}
		dumpBlock(&b, 1, fn.Body)
	}
	for _, entry := range module.EntryPoints {
		line(0, fmt.Sprintf("entry compute %s numthreads(%d,%d,%d)", entry.EmittedName, entry.NumThreadsX, entry.NumThreadsY, entry.NumThreadsZ))
		line(1, fmt.Sprintf("function %s", entry.FunctionName))
		for _, param := range entry.Params {
			line(1, fmt.Sprintf("param %s: %s", param.Name, FormatType(param.Type)))
		}
		for _, thread := range entry.ThreadParams {
			line(1, fmt.Sprintf("thread %s: %s", thread.ParamName, thread.TypeName))
			for _, field := range thread.Fields {
				label := fmt.Sprintf("field %s <- %s", field.FieldName, field.BuiltinName)
				if field.BuiltinField != "" {
					label += "." + field.BuiltinField
				}
				line(2, label)
			}
		}
		for _, builtin := range entry.Builtins {
			line(1, fmt.Sprintf("builtin %s: %s semantic %s referenced=%t", builtin.Name, FormatType(builtin.Type), builtin.Semantic, builtin.Referenced))
		}
	}
	return b.String()
}

func dumpBlock(b *strings.Builder, indent int, block Block) {
	line := func(indent int, text string) {
		for i := 0; i < indent; i++ {
			b.WriteString("  ")
		}
		b.WriteString(text)
		b.WriteByte('\n')
	}
	for _, stmt := range block.Statements {
		switch s := stmt.(type) {
		case LetStmt:
			if s.Value == nil {
				line(indent, fmt.Sprintf("let %s: %s", s.Name, FormatType(s.Type)))
			} else {
				line(indent, fmt.Sprintf("let %s: %s = %s", s.Name, FormatType(s.Type), FormatExpr(s.Value)))
			}
		case AssignStmt:
			line(indent, fmt.Sprintf("assign %s = %s", FormatExpr(s.Target), FormatExpr(s.Value)))
		case TensorAssign:
			parts := make([]string, 0, len(s.FreeIndices))
			for _, index := range s.FreeIndices {
				parts = append(parts, fmt.Sprintf("%s:%d", index.Name, index.Extent))
			}
			line(indent, fmt.Sprintf("tensor %s [%s] %s = %s", FormatExpr(s.Destination), strings.Join(parts, ", "), s.AssignmentKind, FormatExpr(s.Value)))
		case GuardedWriteStmt:
			line(indent, fmt.Sprintf("guarded_write %s = %s when %s", FormatExpr(s.Target), FormatExpr(s.Value), FormatExpr(s.Condition)))
		case ReturnStmt:
			if s.Value == nil {
				line(indent, "return")
			} else {
				line(indent, "return "+FormatExpr(s.Value))
			}
		case ExprStmt:
			line(indent, "expr "+FormatExpr(s.Value))
		case AssertStmt:
			parts := []string{string(s.Kind), fmt.Sprintf("lexical=%d", s.LexicalIndex)}
			if s.Expected != nil {
				parts = append(parts, "expected="+FormatExpr(s.Expected))
			}
			if s.Actual != nil {
				parts = append(parts, "actual="+FormatExpr(s.Actual))
			}
			if s.Tolerance != nil {
				parts = append(parts, "tolerance="+FormatExpr(s.Tolerance))
			}
			line(indent, "assert "+strings.Join(parts, " "))
		case BlockStmt:
			line(indent, "block")
			dumpBlock(b, indent+1, s.Body)
		case IfStmt:
			line(indent, "if "+FormatExpr(s.Condition))
			dumpBlock(b, indent+1, s.ThenBody)
			if s.ElseBody != nil {
				line(indent, "else")
				dumpBlock(b, indent+1, *s.ElseBody)
			}
		case ForRangeStmt:
			hint := ""
			if s.LoopHint != LoopHintNone {
				hint = " [" + string(s.LoopHint) + "]"
			}
			line(indent, fmt.Sprintf("for%s %s: %s in %s..%s step %s", hint, s.Name, FormatType(s.Type), FormatExpr(s.Start), FormatExpr(s.End), FormatExpr(s.Step)))
			dumpBlock(b, indent+1, s.Body)
		}
	}
}

func FormatExpr(expr Expr) string {
	switch e := expr.(type) {
	case LiteralExpr:
		return e.Value
	case VarRefExpr:
		return e.Name
	case FieldAccessExpr:
		return FormatExpr(e.Target) + "." + e.Field
	case IndexExpr:
		return FormatExpr(e.Target) + "[" + FormatExpr(e.Index) + "]"
	case Index2DExpr:
		return FormatExpr(e.Target) + "[" + FormatExpr(e.Row) + ", " + FormatExpr(e.Col) + "]"
	case IndexNExpr:
		parts := make([]string, 0, len(e.Indices))
		for _, index := range e.Indices {
			parts = append(parts, FormatExpr(index))
		}
		return FormatExpr(e.Target) + "[" + strings.Join(parts, ", ") + "]"
	case TensorReductionExpr:
		parts := make([]string, 0, len(e.Indices))
		for _, index := range e.Indices {
			parts = append(parts, fmt.Sprintf("%s:%d", index.Name, index.Extent))
		}
		return e.Kind + "[" + strings.Join(parts, ", ") + "](" + FormatExpr(e.Body) + ")"
	case GuardedReadExpr:
		return "guarded_read(" + FormatExpr(e.Target) + " when " + FormatExpr(e.Condition) + " else " + FormatExpr(e.Fallback) + ")"
	case RegTileZeroExpr:
		return "reg_tile_zero()"
	case RowMajorViewExpr:
		return "row_major(" + FormatExpr(e.Buffer) + ", " + FormatExpr(e.Rows) + ", " + FormatExpr(e.Cols) + ")"
	case CallExpr:
		args := make([]string, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, FormatExpr(arg))
		}
		return FormatExpr(e.Callee) + "(" + strings.Join(args, ", ") + ")"
	case IntrinsicCallExpr:
		args := make([]string, 0, len(e.Arguments))
		for _, arg := range e.Arguments {
			args = append(args, FormatExpr(arg))
		}
		return string(e.Intrinsic) + "(" + strings.Join(args, ", ") + ")"
	case BinaryExpr:
		return "(" + FormatExpr(e.Left) + " " + e.Operator + " " + FormatExpr(e.Right) + ")"
	case UnaryExpr:
		return e.Operator + FormatExpr(e.Operand)
	case WhenUtilityExpr:
		parts := make([]string, 0, len(e.Cases)+1)
		for _, c := range e.Cases {
			parts = append(parts, fmt.Sprintf("case %s when %s score %s", FormatExpr(c.Value), FormatExpr(c.Guard), FormatExpr(c.Score)))
		}
		parts = append(parts, "else "+FormatExpr(e.Else))
		return "when utility { " + strings.Join(parts, " | ") + " }"
	case WithExpr:
		parts := make([]string, 0, len(e.Updates))
		for _, update := range e.Updates {
			parts = append(parts, fmt.Sprintf("%s: %s", update.Name, FormatExpr(update.Value)))
		}
		return FormatExpr(e.Base) + " with { " + strings.Join(parts, ", ") + " }"
	case ReductionExpr:
		hint := ""
		if e.LoopHint != LoopHintNone {
			hint = " [" + string(e.LoopHint) + "]"
		}
		step := ""
		if lit, ok := e.Step.(LiteralExpr); !ok || lit.Value != "1" {
			step = " step " + FormatExpr(e.Step)
		}
		return fmt.Sprintf("%s%s %s in %s..%s%s { %s }", hint, e.Op, e.Name, FormatExpr(e.Start), FormatExpr(e.End), step, FormatExpr(e.Body))
	case EnumConstructExpr:
		if len(e.Fields) == 0 {
			return e.EnumName + "." + e.VariantName
		}
		parts := make([]string, 0, len(e.Fields))
		for _, field := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", field.Name, FormatExpr(field.Value)))
		}
		return e.EnumName + "." + e.VariantName + " { " + strings.Join(parts, ", ") + " }"
	case BoardConstructExpr:
		parts := make([]string, 0, len(e.Fields))
		for _, field := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s: %s", field.Name, FormatExpr(field.Value)))
		}
		return e.TypeName + " { " + strings.Join(parts, ", ") + " }"
	case DeriveExpr:
		parts := make([]string, 0, len(e.Fields))
		for _, field := range e.Fields {
			parts = append(parts, fmt.Sprintf("%s = %s", field.Name, FormatExpr(field.Value)))
		}
		return "derive " + e.TypeName + " { " + strings.Join(parts, "; ") + " }"
	case MatchExpr:
		parts := make([]string, 0, len(e.Arms))
		for _, arm := range e.Arms {
			pattern := arm.EnumName + "." + arm.VariantName
			if arm.BindingName != "" {
				pattern += "(" + arm.BindingName + ")"
			}
			parts = append(parts, pattern+" => "+FormatExpr(arm.Value))
		}
		return "match " + FormatExpr(e.Subject) + " { " + strings.Join(parts, " | ") + " }"
	default:
		return "<expr>"
	}
}

func FormatType(t Type) string {
	switch t.Kind {
	case TypeVoid:
		return "void"
	case TypeBool:
		return "bool"
	case TypeI32:
		return "i32"
	case TypeU32:
		return "u32"
	case TypeF32:
		return "f32"
	case TypeUint2:
		return "uint2"
	case TypeUint3:
		return "uint3"
	case TypeUint4:
		return "uint4"
	case TypeFloat2:
		return formatSpace("float2", t.Space)
	case TypeFloat3:
		return formatSpace("float3", t.Space)
	case TypeFloat4:
		return formatSpace("float4", t.Space)
	case TypeRuntimeArray:
		if t.Element == nil {
			return "array<?>"
		}
		return fmt.Sprintf("array<%s>", FormatType(*t.Element))
	case TypeNDArray:
		if t.Element == nil {
			return "ndarray<?>"
		}
		shape := make([]string, 0, len(t.Shape))
		for _, extent := range t.Shape {
			shape = append(shape, fmt.Sprint(extent))
		}
		return fmt.Sprintf("ndarray<%s,[%s]>", FormatType(*t.Element), strings.Join(shape, ","))
	case TypeArray:
		if t.Element == nil {
			return "array<?>"
		}
		return fmt.Sprintf("array<%s,%d>", FormatType(*t.Element), t.ArraySize)
	case TypeTile:
		if t.Element == nil {
			return "tile<?>"
		}
		return fmt.Sprintf("tile<%s,%d,%d>", FormatType(*t.Element), t.Rows, t.Cols)
	case TypeRegTile:
		if t.Element == nil {
			return "reg_tile<?>"
		}
		return fmt.Sprintf("reg_tile<%s,%d,%d>", FormatType(*t.Element), t.Rows, t.Cols)
	case TypeMatrixView:
		if t.Element == nil {
			return "matrix_view<?>"
		}
		if t.Access != "" {
			return fmt.Sprintf("%s matrix_view<%s>", t.Access, FormatType(*t.Element))
		}
		return fmt.Sprintf("matrix_view<%s>", FormatType(*t.Element))
	case TypeRecord:
		return "record " + t.Name
	case TypeBoard:
		return "board " + t.Name
	case TypeStream:
		return "stream " + t.Name
	default:
		return formatSpace(t.Name, t.Space)
	}
}

func formatSpace(physical, space string) string {
	if space == "" {
		return physical
	}
	return physical + "@space(" + space + ")"
}

func (r Resource) ElementTypeRef() *Type {
	if r.ElementType.Kind == "" {
		return nil
	}
	t := r.ElementType
	return &t
}

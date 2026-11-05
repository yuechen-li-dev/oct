package modules

import (
	"reflect"
	"strings"

	"github.com/yuechen-li-dev/oct/internal/sdslv/ast"
	"github.com/yuechen-li-dev/oct/internal/source"
)

type scope map[string]bool
type binder struct {
	unit *unit
	err  error
}

func cloneScope(parent scope) scope {
	out := scope{}
	for name, bound := range parent {
		out[name] = bound
	}
	return out
}

func (b *binder) local(parent scope, name string, span source.Span) scope {
	out := cloneScope(parent)
	if _, conflict := b.unit.imports[name]; conflict {
		span.Path = b.unit.path
		b.err = fail(span, "SDSL-V4405", "local binding %s conflicts with an import alias", name)
	}
	out[name] = true
	return out
}

func (b *binder) member(alias, name string, span source.Span, kind string) (string, ast.Decl) {
	child := b.unit.imports[alias]
	span.Path = b.unit.path
	decls := child.decls[name]
	var decl ast.Decl
	for _, candidate := range decls {
		if kind == "" || symbolKind(candidate) == kind {
			decl = candidate
			break
		}
	}
	if decl == nil {
		b.err = fail(span, "SDSL-V4407", "namespace %s has no declaration %s", child.module.Namespace, name)
		return name, nil
	}
	_, declaredAt := ast.Declaration(decl)
	exported := false
	for _, exportedAt := range child.module.Exports[name] {
		if exportedAt.Contains(declaredAt) {
			exported = true
			break
		}
	}
	if !exported {
		b.err = fail(span, "SDSL-V4403", "%s.%s is private; export the owning declaration explicitly", child.module.Namespace, name)
		return name, nil
	}
	return child.bound(name, symbolKind(decl)), decl
}

func (b *binder) name(name string, span source.Span, kind string) string {
	parts := strings.Split(name, ".")
	if len(parts) == 2 && b.unit.imports[parts[0]] != nil {
		bound, _ := b.member(parts[0], parts[1], span, kind)
		return bound
	}
	if bound, exists := b.unit.names[name]; exists {
		if kind == "" {
			return bound
		}
		for _, decl := range b.unit.decls[name] {
			if symbolKind(decl) == kind {
				return b.unit.bound(name, kind)
			}
		}
	}
	return name
}

func (b *binder) declaration(decl ast.Decl) (ast.Decl, error) {
	bound := b.value(reflect.ValueOf(decl), scope{}).Interface().(ast.Decl)
	name, _ := ast.Declaration(decl)
	v := reflect.New(reflect.TypeOf(bound)).Elem()
	v.Set(reflect.ValueOf(bound))
	field := v.FieldByName("Name")
	if _, compile := bound.(ast.CompileDecl); compile {
		field = v.FieldByName("AliasName")
	}
	if field.IsValid() && name != "" {
		field.SetString(b.unit.bound(name, symbolKind(decl)))
	}
	return v.Interface().(ast.Decl), b.err
}

func qualified(expr ast.Expr) ([]string, bool) {
	switch e := expr.(type) {
	case ast.IdentifierExpr:
		return []string{e.Name}, true
	case ast.FieldAccessExpr:
		parts, ok := qualified(e.Target)
		return append(parts, e.Field), ok
	default:
		return nil, false
	}
}

func (b *binder) value(v reflect.Value, locals scope) reflect.Value {
	if !v.IsValid() {
		return v
	}
	if v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v
		}
		inner := b.value(v.Elem(), locals)
		out := reflect.New(v.Type()).Elem()
		out.Set(inner)
		return out
	}
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return v
		}
		out := reflect.New(v.Type().Elem())
		out.Elem().Set(b.value(v.Elem(), locals))
		return out
	}
	if v.Kind() == reflect.Slice {
		if v.IsNil() {
			return v
		}
		out := reflect.MakeSlice(v.Type(), v.Len(), v.Len())
		for i := 0; i < v.Len(); i++ {
			out.Index(i).Set(b.value(v.Index(i), locals))
		}
		return out
	}
	if v.Kind() != reflect.Struct {
		return v
	}
	switch d := v.Interface().(type) {
	case source.Span:
		if d.Known() {
			d.Path = b.unit.path
		}
		return reflect.ValueOf(d)
	case ast.IdentifierExpr:
		if !locals[d.Name] {
			d.Name = b.name(d.Name, d.Span, "")
		}
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.TypeRef:
		d.Name = b.name(d.Name, d.Span, "type")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.FieldAccessExpr:
		if parts, ok := qualified(d); ok && len(parts) >= 2 && !locals[parts[0]] && b.unit.imports[parts[0]] != nil {
			name, _ := b.member(parts[0], parts[1], d.Span, "type")
			span := d.Span
			span.Path = b.unit.path
			var expr ast.Expr = ast.IdentifierExpr{Span: span, Name: name}
			for _, member := range parts[2:] {
				expr = ast.FieldAccessExpr{Span: span, Target: expr, Field: member}
			}
			return reflect.ValueOf(expr)
		}
	case ast.CallExpr:
		// A function and a type may share a name. Resolve qualified calls in
		// the function namespace first, as the ordinary validator does.
		if parts, ok := qualified(d.Callee); ok && len(parts) == 2 && !locals[parts[0]] && b.unit.imports[parts[0]] != nil {
			kind := "type"
			for _, decl := range b.unit.imports[parts[0]].decls[parts[1]] {
				if symbolKind(decl) == "function" {
					kind = "function"
					break
				}
			}
			name, _ := b.member(parts[0], parts[1], d.Span, kind)
			span := d.Span
			span.Path = b.unit.path
			return reflect.ValueOf(ast.CallExpr{Span: span, Callee: ast.IdentifierExpr{Span: span, Name: name}, Arguments: b.value(reflect.ValueOf(d.Arguments), locals).Interface().([]ast.Expr)})
		}
		out := b.fields(v, locals, nil).Interface().(ast.CallExpr)
		// Ordinary functions and constructors have declaration-based resolution;
		// SDSL-V does not turn local values into first-class callable objects.
		if id, ok := d.Callee.(ast.IdentifierExpr); ok {
			if name, exists := b.unit.names[id.Name]; exists && !locals[id.Name] {
				for _, decl := range b.unit.decls[id.Name] {
					if symbolKind(decl) == "function" {
						name = b.unit.bound(id.Name, "function")
						break
					}
				}
				id.Name = name
				id.Span.Path = b.unit.path
				out.Callee = id
			}
		}
		return reflect.ValueOf(out)
	case ast.BoardLiteralExpr:
		d.TypeName = b.name(d.TypeName, d.Span, "type")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.EnumConstructExpr:
		if b.unit.imports[d.EnumName] != nil {
			name, decl := b.member(d.EnumName, d.VariantName, d.Span, "type")
			switch decl.(type) {
			case ast.RecordDecl, ast.BoardDecl, ast.StreamDecl, ast.TypeAliasDecl:
				return b.value(reflect.ValueOf(ast.BoardLiteralExpr{Span: d.Span, TypeName: name, Fields: d.Fields}), locals)
			default:
				if b.err == nil {
					span := d.Span
					span.Path = b.unit.path
					b.err = fail(span, "SDSL-V4407", "%s.%s is not an aggregate type", d.EnumName, d.VariantName)
				}
			}
		}
		d.EnumName = b.name(d.EnumName, d.Span, "type")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.MatchArm:
		d.EnumName = b.name(d.EnumName, d.Span, "type")
		child := locals
		if d.BindingName != "" {
			child = b.local(locals, d.BindingName, d.Span)
		}
		return b.fields(reflect.ValueOf(d), locals, map[string]scope{"Value": child})
	case ast.FunctionDecl:
		child := cloneScope(locals)
		for _, parameter := range d.Parameters {
			child = b.local(child, parameter.Name, parameter.Span)
		}
		return b.fields(v, locals, map[string]scope{"Body": child})
	case ast.ShaderDecl:
		bundleName := d.ResourceBundleName
		d.ResourceBundleName = b.name(bundleName, d.Span, "type")
		child := cloneScope(locals)
		if d.Template != nil {
			child = b.local(child, d.Template.Name, d.Template.Span)
		}
		for _, r := range d.Resources {
			child = b.local(child, r.Name, r.Span)
		}
		for _, r := range d.Workgroups {
			child = b.local(child, r.Name, r.Span)
		}
		bundleOwner := b.unit
		if parts := strings.Split(bundleName, "."); len(parts) == 2 && b.unit.imports[parts[0]] != nil {
			bundleOwner, bundleName = b.unit.imports[parts[0]], parts[1]
		}
		for _, decl := range bundleOwner.decls[bundleName] {
			if stream, ok := decl.(ast.StreamDecl); ok {
				for _, field := range stream.Fields {
					child = b.local(child, field.Name, field.Span)
				}
			}
		}
		for _, method := range d.Methods {
			child[method.Name] = true
		}
		return b.fields(reflect.ValueOf(d), child, nil)
	case ast.TemplateParam:
		d.ConceptName = b.name(d.ConceptName, d.Span, "type")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.ConfigDecl:
		d.ConceptName = b.name(d.ConceptName, d.Span, "type")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.CompileDecl:
		d.ShaderName, d.ConfigName = b.name(d.ShaderName, d.Span, "type"), b.name(d.ConfigName, d.Span, "config")
		return b.fields(reflect.ValueOf(d), locals, nil)
	case ast.Block:
		child := cloneScope(locals)
		out := d
		out.Span.Path = b.unit.path
		out.Statements = nil
		for _, stmt := range d.Statements {
			out.Statements = append(out.Statements, b.value(reflect.ValueOf(stmt), child).Interface().(ast.Stmt))
			switch s := stmt.(type) {
			case ast.LetStmt:
				child = b.local(child, s.Name, s.NameSpan)
			case ast.ComptimeLetStmt:
				child = b.local(child, s.Name, s.Span)
			}
		}
		return reflect.ValueOf(out)
	case ast.ForStmt:
		return b.fields(v, locals, map[string]scope{"Body": b.local(locals, d.Name, d.Span)})
	case ast.ComptimeForStmt:
		return b.fields(v, locals, map[string]scope{"Body": b.local(locals, d.Name, d.Span)})
	case ast.ReductionExpr:
		return b.fields(v, locals, map[string]scope{"Body": b.local(locals, d.Name, d.Span)})
	case ast.GenerateExpr:
		child := cloneScope(locals)
		for _, id := range d.Binders {
			child = b.local(child, id.Name, id.Span)
		}
		return b.fields(v, locals, map[string]scope{"Binders": child, "Body": child})
	case ast.TensorAssignStmt:
		child := cloneScope(locals)
		for _, id := range d.FreeIndices {
			child = b.local(child, id.Name, id.Span)
		}
		return b.fields(v, locals, map[string]scope{"Value": child})
	case ast.TensorReductionExpr:
		child := cloneScope(locals)
		for _, id := range d.Indices {
			child = b.local(child, id.Name, id.Span)
		}
		return b.fields(v, locals, map[string]scope{"Value": child})
	case ast.FlowStmt:
		child := cloneScope(locals)
		for _, board := range d.Boards {
			child = b.local(child, board.Name, board.Span)
		}
		return b.fields(v, child, nil)
	}
	return b.fields(v, locals, nil)
}

func (b *binder) fields(v reflect.Value, locals scope, children map[string]scope) reflect.Value {
	out := reflect.New(v.Type()).Elem()
	for i := 0; i < v.NumField(); i++ {
		child := locals
		if own, exists := children[v.Type().Field(i).Name]; exists {
			child = own
		}
		out.Field(i).Set(b.value(v.Field(i), child))
	}
	return out
}

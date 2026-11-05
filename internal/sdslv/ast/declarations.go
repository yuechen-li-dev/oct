package ast

import "github.com/yuechen-li-dev/oct/internal/source"

// Declaration identifies a top-level declaration, independently of its members.
func Declaration(decl Decl) (string, source.Span) {
	switch d := decl.(type) {
	case TypeAliasDecl:
		return d.Name, d.Span
	case RecordDecl:
		return d.Name, d.Span
	case BoardDecl:
		return d.Name, d.Span
	case StreamDecl:
		return d.Name, d.Span
	case ConceptDecl:
		return d.Name, d.Span
	case ConfigDecl:
		return d.Name, d.Span
	case EnumDecl:
		return d.Name, d.Span
	case ShaderDecl:
		return d.Name, d.Span
	case FunctionDecl:
		return d.Name, d.Span
	case CompileDecl:
		return d.AliasName, d.Span
	case UnsupportedDecl:
		return "", d.Span
	default:
		return "", source.Span{}
	}
}
